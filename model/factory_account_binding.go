package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const FactoryAccountBindingStatusEnabled = 1

type FactoryAccountBinding struct {
	Id                 int    `json:"id"`
	FactoryUserId      int    `json:"factory_user_id" gorm:"uniqueIndex;not null"`
	FactoryUsername    string `json:"factory_username" gorm:"type:varchar(64);index"`
	FactoryDisplayName string `json:"factory_display_name" gorm:"type:varchar(64)"`
	FactoryRole        string `json:"factory_role" gorm:"type:varchar(32);default:''"`
	FactoryStatus      int    `json:"factory_status" gorm:"default:1"`
	NewApiUserId       int    `json:"new_api_user_id" gorm:"index;not null"`
	NewApiUsername     string `json:"new_api_username" gorm:"type:varchar(20);index"`
	TokenId            int    `json:"token_id" gorm:"index;not null"`
	Status             int    `json:"status" gorm:"default:1"`
	LastSyncedAt       int64  `json:"last_synced_at" gorm:"bigint"`
	CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt          int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

func (FactoryAccountBinding) TableName() string {
	return "factory_account_bindings"
}

func GetFactoryAccountBindingByFactoryUserId(factoryUserId int) (*FactoryAccountBinding, error) {
	if factoryUserId <= 0 {
		return nil, errors.New("factory_user_id 为空")
	}
	var binding FactoryAccountBinding
	err := DB.Where("factory_user_id = ?", factoryUserId).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func GetFactoryAccountBindingMap(factoryUserIds []int) (map[int]*FactoryAccountBinding, error) {
	result := make(map[int]*FactoryAccountBinding)
	if len(factoryUserIds) == 0 {
		return result, nil
	}
	var bindings []*FactoryAccountBinding
	err := DB.Where("factory_user_id IN ?", factoryUserIds).Find(&bindings).Error
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		result[binding.FactoryUserId] = binding
	}
	return result, nil
}

func GetFactoryAccountBindingMapByTokenIds(tokenIds []int) (map[int]*FactoryAccountBinding, error) {
	result := make(map[int]*FactoryAccountBinding)
	if len(tokenIds) == 0 {
		return result, nil
	}
	var bindings []*FactoryAccountBinding
	err := DB.Where("token_id IN ? AND status = ?", tokenIds, FactoryAccountBindingStatusEnabled).Find(&bindings).Error
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		result[binding.TokenId] = binding
	}
	return result, nil
}

func ListFactoryAccountBindings(offset int, limit int) ([]*FactoryAccountBinding, int64, error) {
	var bindings []*FactoryAccountBinding
	var total int64
	query := DB.Model(&FactoryAccountBinding{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Limit(limit).Offset(offset).Find(&bindings).Error
	return bindings, total, err
}

func SaveFactoryAccountBinding(binding *FactoryAccountBinding) error {
	if binding == nil || binding.FactoryUserId <= 0 {
		return errors.New("factory account binding invalid")
	}
	now := common.GetTimestamp()
	binding.UpdatedAt = now
	if binding.LastSyncedAt == 0 {
		binding.LastSyncedAt = now
	}
	if binding.Status == 0 {
		binding.Status = FactoryAccountBindingStatusEnabled
	}
	var existing FactoryAccountBinding
	err := DB.Where("factory_user_id = ?", binding.FactoryUserId).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			binding.CreatedAt = now
			return DB.Create(binding).Error
		}
		return err
	}
	binding.Id = existing.Id
	binding.CreatedAt = existing.CreatedAt
	return DB.Model(&existing).Select(
		"factory_username",
		"factory_display_name",
		"factory_role",
		"factory_status",
		"new_api_user_id",
		"new_api_username",
		"token_id",
		"status",
		"last_synced_at",
		"updated_at",
	).Updates(binding).Error
}
