package service

import (
	"errors"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"github.com/ghozilaaa/optimistic-lock/models"
)

func UpdateBalance(db *gorm.DB, id uint, delta int64) error {
	const (
		maxAttempts = 5
		baseBackoff = 10 * time.Millisecond
	)

	// Use a local random source for jitter to avoid global Seed usage
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
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
			lastErr = result.Error
		} else if result.RowsAffected == 0 {
			// Conflict: version changed by another transaction
			lastErr = errors.New("conflict: balance updated by another transaction, retry exhausted")
		} else {
			// Success
			if attempt > 1 {
				return errors.New("successful retry")
			}
			return nil
		}

		// If we will retry, sleep with exponential backoff + jitter
		if attempt < maxAttempts {
			// backoff = baseBackoff * 2^(attempt-1)
			backoff := baseBackoff * (1 << (attempt - 1))
			// add jitter up to +-50%
			jitter := time.Duration(rnd.Int63n(int64(backoff))) - backoff/2
			sleep := backoff + jitter
			if sleep < 0 {
				sleep = 0
			}
			time.Sleep(sleep)
			continue
		}

	}

	if lastErr != nil {
		return lastErr
	}

	return errors.New("update failed after retries")
}
