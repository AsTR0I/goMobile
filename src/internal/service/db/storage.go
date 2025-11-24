package db

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Simcard struct {
	ID        string `gorm:"primaryKey;size:36"`
	DefsimID  string `gorm:"size:36"`
	AccountID string `gorm:"size:36"`
	Active    bool
	Iccid     string     `gorm:"-"`
	MvnoIccid *MvnoIccid `gorm:"foreignKey:Iccid;references:Iccid"`
	DefSim    *DefSim    `gorm:"foreignKey:DefsimID;references:ID"`
	Account   *Account   `gorm:"foreignKey:AccountID;references:ID"`
}

type MvnoIccid struct {
	ID             string        `gorm:"primaryKey;size:36"`
	AccountID      string        `gorm:"size:36"`
	HostOperatorID string        `gorm:"size:36"`
	Iccid          string        `gorm:"size:255"`
	HostOperator   *HostOperator `gorm:"foreignKey:HostOperatorID;references:ID"`
}

func (MvnoIccid) TableName() string {
	return "mvno_iccids"
}

type HostOperator struct {
	ID   string `gorm:"primaryKey;size:36"`
	Name string
}

type DefSimTariff struct {
	ID           string      `gorm:"primaryKey;size:36"`
	DefsimID     string      `gorm:"size:36"`
	MvnoTariffID string      `gorm:"size:36"`
	MvnoTariff   *MvnoTariff `gorm:"foreignKey:MvnoTariffID;references:ID"`
}

func (DefSimTariff) TableName() string {
	return "defsim_tariffs"
}

type MvnoTariff struct {
	ID   string `gorm:"primaryKey;size:36"`
	Name string
	Cost float64
}

type DefSim struct {
	ID            string         `gorm:"primaryKey;size:36"`
	AccountID     *string        `gorm:"size:36"`
	MvnoRegionID  *string        `gorm:"size:36"`
	Did           string         `gorm:"size:20;uniqueIndex"`
	Iccid         string         `gorm:"size:191"`
	Imsi          *string        `gorm:"size:191"`
	TfopType      string         `gorm:"size:191"`
	DefSimTariffs []DefSimTariff `gorm:"foreignKey:DefsimID;references:ID"`
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
}

func (DefSim) TableName() string {
	return "defsims"
}

type Account struct {
	ID         string `gorm:"primaryKey;size:36"`
	OperatorID *uint
	NodeID     *string `gorm:"size:4"`
	Name       string
	VirtualPbx *VirtualPbx `gorm:"foreignKey:AccountID;references:ID"`
	Timezone   *string
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
}

type VirtualPbx struct {
	AccountID   string `gorm:"size:36"`
	ID          int
	AccessCode  string
	VoiceNumber string
	PinCode     *string
}

func (VirtualPbx) TableName() string {
	return "account_xvb"
}

type Operator struct {
	ID        uint `gorm:"primaryKey"`
	Name      string
	NodeID    *string `gorm:"size:3"`
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

type Storage interface {
	GetFullSimDataByDid(ctx context.Context, did string) (*Simcard, error)
	GetAccountByID(ctx context.Context, id string) (*Account, error)
	GetOperatorByID(ctx context.Context, id uint) (*Operator, error)
}

type storage struct {
	db *gorm.DB
}

type MySQL struct {
	db *gorm.DB
}
