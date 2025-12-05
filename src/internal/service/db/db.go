package db

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewStorage(host string, port int, user, pass, dbname string) (Storage, error) {

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user,
		pass,
		host,
		port,
		dbname,
	)

	logrus.Debug(dsn)

	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.New(
			logrus.StandardLogger(), // адаптер к logrus
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Silent, // Silent, Info, Warn, Error
				IgnoreRecordNotFoundError: false,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		return nil, err
	}

	return &storage{db: gdb}, nil
}

func (s *storage) GetFullSimDataByDid(ctx context.Context, did string) (*Simcard, error) {
	var sim Simcard

	err := s.db.WithContext(ctx).
		Joins("JOIN defsims ON defsims.id = simcards.defsim_id").
		Where("simcards.active = ?", true).
		Where("defsims.did = ?", did).
		Preload("DefSim").
		Preload("Account").
		Preload("Account.VirtualPbx").
		Preload("DefSim.DefSimTariffs.MvnoTariff").
		First(&sim).Error

	if err != nil {
		return nil, err
	}

	// присваиваем ICCID
	sim.Iccid = sim.DefSim.Iccid

	// грузим MvnoIccid по ICCID
	if err := s.db.WithContext(ctx).
		Preload("HostOperator").
		Where("iccid = ?", sim.Iccid).
		First(&sim.MvnoIccid).Error; err != nil {
		sim.MvnoIccid = nil
	}

	return &sim, nil
}

func (s *storage) GetAccountByID(ctx context.Context, id string) (*Account, error) {
	var acc Account
	if err := s.db.WithContext(ctx).First(&acc, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

func (s *storage) GetOperatorByID(ctx context.Context, id uint) (*Operator, error) {
	var op Operator
	if err := s.db.WithContext(ctx).First(&op, id).Error; err != nil {
		return nil, err
	}
	return &op, nil
}
