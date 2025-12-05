package sipserver

import (
	"context"
	"fmt"
	"gomobile/internal/constants"
	"gomobile/internal/service/logic"
	"net"
	"strings"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type SIPServer struct {
	port   int
	server *sipgo.Server
	logic  *logic.BusinessLogic
}

type SIPMsgMeta struct {
	CallID string
	CSeq   string
	From   string
	To     string
	Method string
	Source string
	Req    *sip.Request
}

func New(port int, bl *logic.BusinessLogic) *SIPServer {
	return &SIPServer{
		port:  port,
		logic: bl,
	}
}

func (s *SIPServer) Start() error {
	logrus.Infof("Starting SIP server (sipgo) on UDP :%d", s.port)

	// создаётся сервер
	ua, err := sipgo.NewUA()
	if err != nil {
		logrus.Error("Fail to setup user agent", "error", err)
		return nil
	}

	s.server, err = sipgo.NewServer(ua)
	if err != nil {
		return err
	}

	// хендлеры
	s.server.OnOptions(s.wrapWithACL(s.handleOptions))
	s.server.OnInvite(s.wrapWithACL(s.handleInvite))
	s.server.OnCancel(s.wrapWithACL(s.handleCancel))
	s.server.OnAck(s.wrapWithACL(s.handleAck))
	s.server.OnBye(s.wrapWithACL(s.handleBye))

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)

	// start UDP
	ctx := context.Background()
	go func() {
		if err := s.server.ListenAndServe(ctx, "udp", addr); err != nil {
			logrus.Fatalf("SIP Listen error: %v", err)
		}
	}()

	return nil
}

func (s *SIPServer) parseRequest(req *sip.Request) *SIPMsgMeta {
	callID := ""
	if h := req.CallID(); h != nil {
		callID = h.Value()
	}

	cseq := ""
	if h := req.CSeq(); h != nil {
		cseq = h.String()
	}

	from := ""
	if h := req.From(); h != nil {
		from = h.Address.String()
	}

	to := ""
	if h := req.To(); h != nil {
		to = h.Address.String()
	}

	return &SIPMsgMeta{
		CallID: callID,
		CSeq:   cseq,
		From:   from,
		To:     to,
		Method: req.Method.String(),
		Source: req.Source(),
		Req:    req,
	}
}

func (s *SIPServer) handleOptions(req *sip.Request, tx sip.ServerTransaction) {
	start := time.Now()

	meta := s.parseRequest(req)
	logrus.Infof("Call-ID: %s OPTIONS from %s", meta.CallID, meta.Source)

	resp := sip.NewResponseFromRequest(req, 200, "OK", nil)
	resp.AppendHeader(sip.NewHeader("X-Elapsed-Time", fmt.Sprintf("%.3fms", float64(time.Since(start).Microseconds())/1000)))
	s.decorateResponse(resp)
	if err := tx.Respond(resp); err != nil {
		logrus.Errorf("Call-ID: %s Failed to send OPTIONS: %v", meta.CallID, err)
	}
}

func (s *SIPServer) handleInvite(req *sip.Request, tx sip.ServerTransaction) {
	meta := s.parseRequest(req)
	logrus.Infof("Call-ID: %s INVITE received from %s", meta.CallID, meta.Source)
	start := time.Now()

	// 100 Trying
	{
		resp100 := sip.NewResponseFromRequest(req, 100, "Trying", nil)
		resp100.AppendHeader(sip.NewHeader("X-Elapsed-Time", fmt.Sprintf("%.3fms", float64(time.Since(start).Microseconds())/1000)))
		s.decorateResponse(resp100)
		_ = tx.Respond(resp100)
		logrus.Infof("Call-ID: %s 100 Trying sent", meta.CallID)
	}

	// 302 Redirect (будет ретранслироваться автоматически на UDP до получения ACK)
	{
		ruri := req.Recipient.Endpoint()
		// logrus.Infof("Call-ID: %s R-URI: %s", meta.CallID, ruri)

		numA, numB, numC, callID, srcIP, sbcIP, start, err := extractInviteData(req)
		if err != nil {
			logrus.Errorf("Call-ID: %s Failed to extract INVITE data: %v", meta.CallID, err)
			resp := sip.NewResponseFromRequest(req, 502, "Bad Request", nil)
			s.decorateResponse(resp)
			if err := tx.Respond(resp); err != nil {
				logrus.Errorf("Call-ID: %s Failed to send 400: %v", callID, err)
			}
			return
		}
		unixTime := time.Now().Unix()

		result := s.logic.FindPolicyResult(numA, numB, numC, srcIP, sbcIP, callID, ruri, unixTime)

		switch result.Target {
		case "Bad Gateway":
			resp := sip.NewResponseFromRequest(req, 502, "Bad Gateway", nil)
			resp.AppendHeader(sip.NewHeader("Reason", result.Reason))
			s.decorateResponse(resp)
			resp.AppendHeader(sip.NewHeader("X-Elapsed-Time", fmt.Sprintf("%.3fms", float64(time.Since(start).Microseconds())/1000)))

			if err := tx.Respond(resp); err != nil {
				logrus.Errorf("Call-ID: %s Failed to send 502: %v", callID, err)
			}
		default:
			contacts := strings.Split(result.Target, "|")
			resp := sip.NewResponseFromRequest(req, 302, "Moved Temporarily", nil)

			for _, c := range contacts {
				c = strings.TrimSpace(c)
				// Убираем лишний префикс Contact:, если есть
				if strings.HasPrefix(c, "Contact:") {
					c = strings.TrimSpace(c[8:])
				} else if strings.HasPrefix(c, "contact:") {
					c = strings.TrimSpace(c[8:])
				}
				if c != "" {
					resp.AppendHeader(sip.NewHeader("Contact", c))
				}
			}

			s.decorateResponse(resp)
			resp.AppendHeader(sip.NewHeader("X-Elapsed-Time", fmt.Sprintf("%.3fms", float64(time.Since(start).Microseconds())/1000)))

			if err := tx.Respond(resp); err != nil {
				logrus.Errorf("Call-ID: %s Failed to send 302: %v", callID, err)
			} else {
				logrus.Infof("Call-ID: %s 302 Redirect sent to %s", callID, result.Target)
			}
		}
	}
}

func (s *SIPServer) handleCancel(req *sip.Request, tx sip.ServerTransaction) {
	meta := s.parseRequest(req)
	logrus.Infof("Call-ID: %s CANCEL received from %s", meta.CallID, meta.Source)
	resp := sip.NewResponseFromRequest(req, 200, "OK", nil)
	s.decorateResponse(resp)
	tx.Respond(resp)

	logrus.Infof("Call-ID: %s 200 OK sent", meta.CallID)
}

func (s *SIPServer) handleAck(req *sip.Request, tx sip.ServerTransaction) {
	meta := s.parseRequest(req)
	logrus.Infof("Call-ID: %s ACK received from %s", meta.CallID, meta.Source)
}

func (s *SIPServer) handleBye(req *sip.Request, tx sip.ServerTransaction) {
	meta := s.parseRequest(req)
	logrus.Infof("Call-ID: %s BYE received from %s", meta.CallID, meta.Source)
}

// helpers

func (s *SIPServer) decorateResponse(resp *sip.Response) {
	resp.AppendHeader(
		sip.NewHeader("Server", constants.AppName+constants.SPACE+constants.Version),
	)
}

// wrapWithACL создаёт новый хендлер с проверкой ACL
func (s *SIPServer) wrapWithACL(handler func(*sip.Request, sip.ServerTransaction)) func(*sip.Request, sip.ServerTransaction) {
	return func(req *sip.Request, tx sip.ServerTransaction) {
		src := req.Source()
		if !isIPAllowed(src) {
			logrus.Warnf("ACL deny %s from %s", req.Method.String(), src)
			resp := sip.NewResponseFromRequest(req, 603, "Decline", nil)
			resp.AppendHeader(sip.NewHeader("Reason", "Access denied by ACL"))
			s.decorateResponse(resp)
			_ = tx.Respond(resp)
			return
		}
		handler(req, tx)
	}
}
func isIPAllowed(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		logrus.Infof("Invalid remote address: %v", addr)
		return false
	}

	for _, allowedIP := range viper.GetStringSlice("sipserver.acl.ip") {
		if allowedIP == host {
			return true
		}
	}
	return false
}

func extractInviteData(req *sip.Request) (numA, numB, numC, callID, srcIP, sbcIP string, start time.Time, err error) {
	start = time.Now()
	callID = req.CallID().Value()

	// IP источника и SBC (без порта)
	if srcAddr := req.Source(); srcAddr != "" {
		srcIP = strings.Split(srcAddr, ":")[0]
		sbcIP = srcIP
	} else {
		srcIP = ""
		sbcIP = ""
	}

	// From
	fromHeader := req.From()
	if fromHeader != nil {
		numA = fromHeader.Address.User
	} else {
		err = fmt.Errorf("missing From header")
		return
	}

	// To
	toHeader := req.To()
	if toHeader != nil {
		numB = toHeader.Address.User
	} else {
		err = fmt.Errorf("missing To header")
		return
	}

	diversionHeader := req.GetHeader("Diversion")
	if diversionHeader != nil {
		numC = diversionHeader.Value()
		numC = strings.Split(numC, ",")[0]
		numC = strings.Trim(numC, "<>")
		numC = ExtractNumber(numC)
	} else {
		numC = ""
	}

	return
}

func ExtractNumber(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "sip:")
	s = strings.TrimPrefix(s, "tel:")
	s = strings.TrimPrefix(s, "+")

	if at := strings.Index(s, "@"); at != -1 {
		s = s[:at]
	}

	return s
}
