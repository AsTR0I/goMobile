package http

import (
	"fmt"
	"strings"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/gin-gonic/gin"
)

func (h *HTTPServer) initRoutes() {
	h.engine.GET("/simulation", h.handleSimulate)
}

// SIPHeader представляет один SIP-заголовок
// @Description SIP Header pair
type SIPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// InviteDebugResponse возвращает результат симуляции
// @Description Parsed SIP request and response with raw packets
type InviteDebugResponse struct {
	Data struct {
		InviteHeaders   []SIPHeader `json:"invite"`
		ResponseHeaders []SIPHeader `json:"result"`
	} `json:"data"`
}

// Парсинг SIP-заголовков из строки
func ParseSIPHeaders(packet string) []SIPHeader {
	var headers []SIPHeader
	lines := strings.Split(packet, "\r\n")
	if len(lines) > 0 {
		headers = append(headers, SIPHeader{Name: "Method", Value: lines[0]})
	}

	headerNames := []string{
		"From", "To", "Call-ID", "CSeq", "Contact", "User-Agent", "Diversion",
		"Content-Length", "X-SrcIP", "Reason", "X-Elapsed-Time",
	}

	for _, line := range lines {
		for _, h := range headerNames {
			if len(line) > 0 && (len(line) > len(h)+1) && line[:len(h)+1] == h+":" {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					headers = append(headers, SIPHeader{Name: h, Value: strings.TrimSpace(parts[1])})
				}
			}
		}
	}

	return headers
}

// @Summary      SIP Policy Simulation
// @Description  Выполняет симуляцию SIP INVITE, прогоняет его через бизнес-логику Policy (logic) и возвращает INVITE/RESPONSE в сыром и разобранном виде.
// @Tags         Simulation
// @Accept       json
// @Produce      json
// @Param        a_number   query string true  "A-number (From)"
// @Param        b_number   query string true  "B-number (To)"
// @Param        c_number   query string false "C-number (Diversion)"
// @Param        src_ip     query string true  "Источник SIP пакета (X-SrcIP)"
// @Param        sbc_ip     query string false "IP SBC (по умолчанию 0.0.0.0)"
// @Success      200 {object} InviteDebugResponse "Успешная симуляция"
// @Failure      500 {object} map[string]string  "Ошибка обработки URI или другого системного компонента"
// @Router       /simulation [get]
func (h *HTTPServer) handleSimulate(c *gin.Context) {
	start := time.Now()

	aNumber := c.Query("a_number")
	bNumber := c.Query("b_number")
	cNumber := c.Query("c_number")
	srcIP := c.Query("src_ip")
	sbcIP := c.Query("sbc_ip")
	if sbcIP == "" {
		sbcIP = "0.0.0.0"
	}

	callID := fmt.Sprintf("cid-%d", time.Now().UnixNano())

	var uri sip.Uri
	err := sip.ParseUri(fmt.Sprintf("sip:%s@%s", bNumber, sbcIP), &uri)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to parse SIP URI"})
		return
	}

	reqMsg := sip.NewRequest(sip.INVITE, uri)

	reqMsg.AppendHeader(sip.NewHeader("From", fmt.Sprintf("<sip:%s>", aNumber)))
	reqMsg.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<sip:%s>", bNumber)))
	reqMsg.AppendHeader(sip.NewHeader("Call-ID", callID))
	reqMsg.AppendHeader(sip.NewHeader("CSeq", "1 INVITE"))
	reqMsg.AppendHeader(sip.NewHeader("Contact", fmt.Sprintf("<sip:%s@%s:53799;transport=udp>", aNumber, srcIP)))
	reqMsg.AppendHeader(sip.NewHeader("User-Agent", "PolicySimulation/1.0"))
	reqMsg.AppendHeader(sip.NewHeader("X-SrcIP", srcIP))
	if cNumber != "" {
		reqMsg.AppendHeader(sip.NewHeader("Diversion", fmt.Sprintf("<sip:%s>", cNumber)))
	}

	unixTime := time.Now().Unix()
	result := h.logic.FindPolicyResult(aNumber, bNumber, cNumber, srcIP, sbcIP, callID, unixTime)

	var respMsg *sip.Response
	elapsed := fmt.Sprintf("%dms", time.Since(start).Milliseconds())

	if result.Target == "Bad Gateway" {
		respMsg = sip.NewResponseFromRequest(reqMsg, 502, "Bad Gateway", nil)
		respMsg.AppendHeader(sip.NewHeader("Reason", result.Reason))
		respMsg.AppendHeader(sip.NewHeader("X-Elapsed-Time", elapsed))
	} else {
		respMsg = sip.NewResponseFromRequest(reqMsg, 302, "Moved Temporarily", nil)
		respMsg.AppendHeader(sip.NewHeader("Contact", result.Target))
		respMsg.AppendHeader(sip.NewHeader("X-Elapsed-Time", elapsed))
	}

	sipResponse := respMsg.String()

	mockPacket := reqMsg.String()

	response := InviteDebugResponse{}
	response.Data.InviteHeaders = ParseSIPHeaders(mockPacket)
	response.Data.ResponseHeaders = ParseSIPHeaders(sipResponse)

	c.JSON(200, gin.H{
		"invite_packet_raw": mockPacket,
		"sip_response_raw":  sipResponse,
		"data":              response.Data,
	})
}
