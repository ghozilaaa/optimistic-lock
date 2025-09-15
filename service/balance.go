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

	originalVersion := balance.Version
	balance.Amount += delta

	// Use UPDATE with WHERE clause to check version for optimistic locking
	result := db.Model(&balance).Where("id = ? AND version = ?", balance.ID, originalVersion).Updates(models.Balance{
		Amount:  balance.Amount,
		Version: balance.Version + 1,
	})

	if result.Error != nil {
		return result.Error
	}

	// If no rows were affected, it means the version was already updated by another transaction
	if result.RowsAffected == 0 {
		return errors.New("conflict: balance updated by another transaction")
	}

	return nil
}
