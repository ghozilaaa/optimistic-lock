package service_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ghozilaaa/optimistic-lock/models"
	"github.com/ghozilaaa/optimistic-lock/service"
)

func TestConcurrentBalanceUpdates(t *testing.T) {
	dsn := "host=localhost user=postgres dbname=optimistic_lock password=postgres sslmode=disable"
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true, // disable default transaction for write operations
		Logger:                 logger.Default.LogMode(logger.Silent),
		PrepareStmt:            true, // creates a prepared statement when executing any SQL and caches them to speed up future calls
	})

	db.AutoMigrate(&models.Balance{})
	db.Exec("DELETE FROM balances") // Clear for test

	// Seed with initial balance
	balance := models.Balance{Amount: 1000}
	db.Create(&balance)

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	now := time.Now()

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

	elapsed := time.Since(now)
	t.Logf("All updates completed in %s", elapsed)

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

// TPSTestConfig holds configuration for TPS performance tests
type TPSTestConfig struct {
	Name        string  // Test name
	TargetTPS   int     // Target transactions per second
	Duration    int     // Test duration in seconds
	AmountPerTx int64   // Amount to add per transaction
	MinTPS      float64 // Minimum acceptable TPS
	MaxTPS      float64 // Maximum acceptable TPS
	LogFailures int     // Number of failures to log (0 = log all)
}

// runTPSTest executes a configurable TPS test
func runTPSTest(t *testing.T, config TPSTestConfig) {
	dsn := "host=localhost user=postgres dbname=optimistic_lock password=postgres sslmode=disable"
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true, // disable default transaction for write operations
		Logger:                 logger.Default.LogMode(logger.Silent),
		PrepareStmt:            true, // creates a prepared statement when executing any SQL and caches them to speed up future calls
	})

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get database: %v", err)
	}

	// Set pool configuration
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(30 * time.Second)

	db.AutoMigrate(&models.Balance{})
	db.Exec("DELETE FROM balances") // Clear for test

	// Seed with initial balance
	balance := models.Balance{Amount: 1000}
	db.Create(&balance)

	duration := time.Duration(config.Duration) * time.Second
	totalTransactions := config.TargetTPS * config.Duration
	interval := duration / time.Duration(totalTransactions)

	var wg sync.WaitGroup
	errs := make(chan error, totalTransactions)

	now := time.Now()
	t.Logf("Starting %s: %d transactions over %v (interval: %v, target TPS: %d)",
		config.Name, totalTransactions, duration, interval, config.TargetTPS)

	// Launch transactions at controlled rate
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	transactionCount := 0
	failureCount := 0

	for range ticker.C {
		if transactionCount >= totalTransactions {
			break
		}

		wg.Add(1)
		go func(txNum int) {
			defer wg.Done()
			start := time.Now()
			err := service.UpdateBalance(db, balance.ID, config.AmountPerTx)
			txDuration := time.Since(start)

			if err != nil {
				errs <- err
				// Log failures based on config
				if config.LogFailures == 0 || failureCount < config.LogFailures {
					t.Logf("Transaction %d failed after %v: %v", txNum, txDuration, err)
					failureCount++
				}
			}
		}(transactionCount + 1)

		transactionCount++
	}

	wg.Wait()
	close(errs)

	elapsed := time.Since(now)

	// Count conflicts and other errors
	conflictCount := 0
	successfulRetry := 0
	otherErrorCount := 0
	for err := range errs {
		if err != nil {
			if err.Error() == "conflict: balance updated by another transaction, retry exhausted" {
				conflictCount++
			} else if err.Error() == "successful retry" {
				successfulRetry++
			} else {
				otherErrorCount++
			}
		}
	}

	successfulTransactions := totalTransactions - conflictCount - otherErrorCount
	actualTPS := float64(successfulTransactions) / elapsed.Seconds()
	t.Logf("Test completed in %v, Actual TPS: %.2f", elapsed, actualTPS)

	// Reload balance
	var updated models.Balance
	db.First(&updated, balance.ID)
	t.Logf("Final balance: %d, successful: %d, conflicts: %d, other errors: %d, successful retry %d",
		updated.Amount, successfulTransactions, conflictCount, otherErrorCount, successfulRetry)

	// Verify balance integrity
	expected := int64(1000 + (int64(successfulTransactions) * config.AmountPerTx))
	if updated.Amount != expected {
		t.Errorf("Balance integrity failed: expected %d, got %d", expected, updated.Amount)
	}

	// Verify TPS is within acceptable range
	if actualTPS < config.MinTPS || actualTPS > config.MaxTPS {
		t.Logf("Warning: TPS variance outside expected range. Target: %d, Actual: %.2f (Range: %.1f-%.1f)",
			config.TargetTPS, actualTPS, config.MinTPS, config.MaxTPS)
	}

	// Log detailed metrics
	conflictRate := float64(conflictCount) / float64(totalTransactions) * 100
	successRate := float64(successfulTransactions) / float64(totalTransactions) * 100
	avgTxDuration := elapsed / time.Duration(totalTransactions)

	t.Logf("Performance Metrics:")
	t.Logf("  - Success rate: %.2f%% (%d/%d)", successRate, successfulTransactions, totalTransactions)
	t.Logf("  - Conflict rate: %.2f%% (%d/%d)", conflictRate, conflictCount, totalTransactions)
	t.Logf("  - Average transaction duration: %v", avgTxDuration)
}

// TestConfigurableTPSScenarios runs multiple TPS test scenarios
func TestConfigurableTPSScenarios(t *testing.T) {
	testCases := []TPSTestConfig{
		{
			Name:        "400 TPS Stress Test 1",
			TargetTPS:   400,
			Duration:    10,
			AmountPerTx: 1,
			MinTPS:      380.0,
			MaxTPS:      400.0,
			LogFailures: 3,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			runTPSTest(t, testCase)
		})
	}
}

// TestSingleTPS allows running a single TPS configuration for quick testing
func TestSingleTPS(t *testing.T) {
	// Modify this configuration as needed for quick single tests
	config := TPSTestConfig{
		Name:        "Custom TPS Test",
		TargetTPS:   25,
		Duration:    5, // Shorter duration for quick testing
		AmountPerTx: 4,
		MinTPS:      20.0,
		MaxTPS:      30.0,
		LogFailures: 5,
	}

	runTPSTest(t, config)
}

// TestVariableIntervalTPS tests with non-static intervals to simulate realistic traffic
func TestVariableIntervalTPS(t *testing.T) {
	dsn := "host=localhost user=postgres dbname=optimistic_lock password=postgres sslmode=disable"
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	db.AutoMigrate(&models.Balance{})
	db.Exec("DELETE FROM balances") // Clear for test

	// Seed with initial balance
	balance := models.Balance{Amount: 1000}
	db.Create(&balance)

	var wg sync.WaitGroup
	targetTPS := 20
	duration := 10 * time.Second
	totalTransactions := targetTPS * 10

	// Base interval for target TPS
	baseInterval := duration / time.Duration(totalTransactions)

	errs := make(chan error, totalTransactions)
	transactionTimes := make([]time.Time, 0, totalTransactions)

	now := time.Now()
	t.Logf("Starting Variable Interval TPS test: target %d TPS with dynamic intervals", targetTPS)
	t.Logf("Base interval: %v (will vary ±50%%)", baseInterval)

	// Launch transactions with variable intervals inline to avoid a race where
	// WaitGroup.Add is called after Wait has begun (which can panic).
	transactionCount := 0
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for transactionCount < totalTransactions {
		// Vary interval by ±50% of base interval
		variance := rng.Float64() - 0.5 // -0.5 to +0.5
		variableInterval := time.Duration(float64(baseInterval) * (1.0 + variance))

		// Ensure minimum interval of 1ms to prevent overwhelming the system
		if variableInterval < time.Millisecond {
			variableInterval = time.Millisecond
		}

		time.Sleep(variableInterval)

		wg.Add(1)
		txTime := time.Now()
		transactionTimes = append(transactionTimes, txTime)

		go func(txNum int, startTime time.Time) {
			defer wg.Done()
			err := service.UpdateBalance(db, balance.ID, 3)
			txDuration := time.Since(startTime)

			if err != nil {
				errs <- err
				if txNum <= 5 { // Log first 5 failures
					t.Logf("Transaction %d failed after %v: %v", txNum, txDuration, err)
				}
			}
		}(transactionCount+1, txTime)

		transactionCount++
	}

	wg.Wait()
	close(errs)

	elapsed := time.Since(now)
	actualTPS := float64(totalTransactions) / elapsed.Seconds()

	// Analyze interval distribution
	intervals := make([]time.Duration, 0, len(transactionTimes)-1)
	var totalInterval time.Duration
	// Initialize min/max to zero values and only update them when we have
	// at least one interval. This avoids using absurd initial values if no
	// intervals were recorded.
	var minInterval time.Duration
	var maxInterval time.Duration

	if len(transactionTimes) >= 2 {
		for i := 1; i < len(transactionTimes); i++ {
			interval := transactionTimes[i].Sub(transactionTimes[i-1])
			intervals = append(intervals, interval)
			totalInterval += interval

			if i == 1 {
				minInterval = interval
				maxInterval = interval
			} else {
				if interval < minInterval {
					minInterval = interval
				}
				if interval > maxInterval {
					maxInterval = interval
				}
			}
		}
	}

	var avgInterval time.Duration
	if len(intervals) > 0 {
		avgInterval = totalInterval / time.Duration(len(intervals))
	} else {
		avgInterval = 0
	}

	t.Logf("Test completed in %v, Actual TPS: %.2f", elapsed, actualTPS)
	t.Logf("Interval Statistics:")
	t.Logf("  - Base interval: %v", baseInterval)
	t.Logf("  - Average interval: %v", avgInterval)
	t.Logf("  - Min interval: %v", minInterval)
	t.Logf("  - Max interval: %v", maxInterval)
	t.Logf("  - Interval variance: %.2f%% of base",
		float64(maxInterval-minInterval)/float64(baseInterval)*100)

	// Count conflicts and errors
	conflictCount := 0
	otherErrorCount := 0
	for err := range errs {
		if err != nil {
			if err.Error() == "conflict: balance updated by another transaction" {
				conflictCount++
			} else {
				otherErrorCount++
			}
		}
	}

	successfulTransactions := totalTransactions - conflictCount - otherErrorCount

	// Reload balance
	var updated models.Balance
	db.First(&updated, balance.ID)
	t.Logf("Final balance: %d, successful: %d, conflicts: %d, other errors: %d",
		updated.Amount, successfulTransactions, conflictCount, otherErrorCount)

	// Verify balance integrity
	expected := int64(1000 + (int64(successfulTransactions) * 3))
	if updated.Amount != expected {
		t.Errorf("Balance integrity failed: expected %d, got %d", expected, updated.Amount)
	}

	// Calculate and log performance metrics
	conflictRate := float64(conflictCount) / float64(totalTransactions) * 100
	successRate := float64(successfulTransactions) / float64(totalTransactions) * 100

	t.Logf("Performance with Variable Intervals:")
	t.Logf("  - Success rate: %.2f%% (%d/%d)", successRate, successfulTransactions, totalTransactions)
	t.Logf("  - Conflict rate: %.2f%% (%d/%d)", conflictRate, conflictCount, totalTransactions)
	t.Logf("  - TPS variance from target: %.2f%% (target: %d, actual: %.2f)",
		(actualTPS-float64(targetTPS))/float64(targetTPS)*100, targetTPS, actualTPS)
}

// TestBurstTrafficPattern simulates burst traffic patterns
func TestBurstTrafficPattern(t *testing.T) {
	dsn := "host=localhost user=postgres dbname=optimistic_lock password=postgres sslmode=disable"
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	db.AutoMigrate(&models.Balance{})
	db.Exec("DELETE FROM balances") // Clear for test

	// Seed with initial balance
	balance := models.Balance{Amount: 1000}
	db.Create(&balance)

	var wg sync.WaitGroup
	errs := make(chan error, 200)

	now := time.Now()
	t.Logf("Starting Burst Traffic Pattern test")

	// Simulate burst pattern: high activity, then low activity
	burstPatterns := []struct {
		name       string
		count      int
		interval   time.Duration
		pauseAfter time.Duration
	}{
		{"High Burst", 50, 10 * time.Millisecond, 2 * time.Second},
		{"Medium Activity", 30, 100 * time.Millisecond, 1 * time.Second},
		{"Low Activity", 20, 200 * time.Millisecond, 500 * time.Millisecond},
		{"Final Burst", 40, 20 * time.Millisecond, 0},
	}

	totalTransactions := 0
	for _, pattern := range burstPatterns {
		totalTransactions += pattern.count
	}

	transactionNum := 0

	for _, pattern := range burstPatterns {
		t.Logf("Starting %s: %d transactions with %v intervals",
			pattern.name, pattern.count, pattern.interval)

		for i := 0; i < pattern.count; i++ {
			wg.Add(1)
			transactionNum++

			go func(txNum int) {
				defer wg.Done()
				start := time.Now()
				err := service.UpdateBalance(db, balance.ID, 2)
				txDuration := time.Since(start)

				if err != nil {
					errs <- err
					if txNum <= 3 { // Log first few failures per burst
						t.Logf("Transaction %d failed after %v: %v", txNum, txDuration, err)
					}
				}
			}(transactionNum)

			if i < pattern.count-1 { // Don't sleep after last transaction in pattern
				time.Sleep(pattern.interval)
			}
		}

		if pattern.pauseAfter > 0 {
			t.Logf("Pausing for %v between burst patterns", pattern.pauseAfter)
			time.Sleep(pattern.pauseAfter)
		}
	}

	wg.Wait()
	close(errs)

	elapsed := time.Since(now)
	actualTPS := float64(totalTransactions) / elapsed.Seconds()

	t.Logf("Burst test completed in %v, Average TPS: %.2f", elapsed, actualTPS)

	// Count conflicts and errors
	conflictCount := 0
	otherErrorCount := 0
	for err := range errs {
		if err != nil {
			if err.Error() == "conflict: balance updated by another transaction" {
				conflictCount++
			} else {
				otherErrorCount++
			}
		}
	}

	successfulTransactions := totalTransactions - conflictCount - otherErrorCount

	// Reload balance
	var updated models.Balance
	db.First(&updated, balance.ID)
	t.Logf("Final balance: %d, successful: %d, conflicts: %d",
		updated.Amount, successfulTransactions, conflictCount)

	// Verify balance integrity
	expected := int64(1000 + (int64(successfulTransactions) * 2))
	if updated.Amount != expected {
		t.Errorf("Balance integrity failed: expected %d, got %d", expected, updated.Amount)
	}

	// Performance analysis
	conflictRate := float64(conflictCount) / float64(totalTransactions) * 100
	t.Logf("Burst Traffic Results:")
	t.Logf("  - Total transactions: %d", totalTransactions)
	t.Logf("  - Success rate: %.2f%%", float64(successfulTransactions)/float64(totalTransactions)*100)
	t.Logf("  - Conflict rate: %.2f%%", conflictRate)
	t.Logf("  - Average TPS across all bursts: %.2f", actualTPS)
}
