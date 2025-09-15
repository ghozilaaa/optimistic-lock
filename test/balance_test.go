package service_test

import (
	"sync"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/ghozilaaa/optimistic-lock/models"
	"github.com/ghozilaaa/optimistic-lock/service"
)

func TestConcurrentBalanceUpdates(t *testing.T) {
	dsn := "host=localhost user=postgres dbname=optimistic_lock password=postgres sslmode=disable"
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	db.AutoMigrate(&models.Balance{})
	db.Exec("DELETE FROM balances") // Clear for test

	// Seed with initial balance
	balance := models.Balance{Amount: 1000}
	db.Create(&balance)

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	// 100 concurrent updates: add 10 to balance
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := service.UpdateBalance(db, balance.ID, 10)
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	// Check for conflicts
	conflictCount := 0
	for err := range errs {
		if err != nil && err.Error() == "conflict: balance updated by another transaction" {
			conflictCount++
		}
	}

	// Reload balance
	var updated models.Balance
	db.First(&updated, balance.ID)
	t.Logf("Final balance: %d, conflicts: %d", updated.Amount, conflictCount)

	// With optimistic locking, some updates may fail due to conflict. Check integrity:
	expected := int64(1000 + ((100 - conflictCount) * 10))
	if updated.Amount != expected {
		t.Errorf("Balance integrity failed: expected %d, got %d", expected, updated.Amount)
	}
}
