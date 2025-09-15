package service

import (
	"errors"

	"gorm.io/gorm"

	"github.com/ghozilaaa/optimistic-lock/models"
)

func UpdateBalance(db *gorm.DB, id uint, delta int64) error {
	var balance models.Balance
	if err := db.First(&balance, id).Error; err != nil {
		return err
	}

	balance.Amount += delta
	err := db.Save(&balance).Error

	if errors.Is(err, gorm.ErrCheckConstraintViolated) {
		return errors.New("conflict: balance updated by another transaction")
	}
	return err
}
