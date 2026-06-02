// =============================================================================
// FX REVALUATION SERVICE
//
// MISSION: Manage foreign exchange rates and revaluation for multi-currency accounts
// FEATURES: FX rate management, unrealized gain/loss calculation, CBB rates import
// =============================================================================

package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// =============================================================================
// FX RATE MANAGEMENT
// =============================================================================

// CreateFXRate adds a new exchange rate
func (a *App) CreateFXRate(fromCurrency, toCurrency string, rate float64, rateDate time.Time, source string) (*FXRate, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Absolute bounds: realistic FX rate range regardless of history
	const minFXRate = 0.001  // e.g. very weak currencies vs BHD
	const maxFXRate = 1000.0 // hard ceiling to catch typos / injected values
	if rate <= 0 {
		return nil, fmt.Errorf("rate must be positive")
	}
	if rate < minFXRate || rate > maxFXRate {
		return nil, fmt.Errorf("rate %.6f is outside absolute bounds [%.3f, %.1f]", rate, minFXRate, maxFXRate)
	}

	// Relative bounds: rate must be within ±20% of last known rate (prevents large sudden jumps)
	var lastRate FXRate
	lastErr := a.db.Where("from_currency = ? AND to_currency = ?", fromCurrency, toCurrency).
		Order("rate_date DESC").First(&lastRate).Error
	if lastErr == nil && lastRate.Rate > 0 {
		ratio := rate / lastRate.Rate
		if ratio < 0.80 || ratio > 1.20 {
			return nil, fmt.Errorf("rate %.6f is outside ±20%% of last known rate %.6f (bounds: %.6f - %.6f)",
				rate, lastRate.Rate, lastRate.Rate*0.80, lastRate.Rate*1.20)
		}
	}

	// Check for existing rate on same date
	var existing FXRate
	err := a.db.Where("from_currency = ? AND to_currency = ? AND rate_date = ?",
		fromCurrency, toCurrency, rateDate).First(&existing).Error
	if err == nil {
		// Update existing rate
		existing.Rate = rate
		existing.Source = source
		if err := a.db.Save(&existing).Error; err != nil {
			return nil, fmt.Errorf("failed to update rate: %w", err)
		}
		log.Printf("📊 FX rate updated: %s/%s = %.6f on %s", fromCurrency, toCurrency, rate, rateDate.Format("2006-01-02"))
		return &existing, nil
	}

	fxRate := &FXRate{
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
		RateDate:     rateDate,
		Rate:         rate,
		Source:       source,
	}

	if err := a.db.Create(fxRate).Error; err != nil {
		return nil, fmt.Errorf("failed to create rate: %w", err)
	}

	log.Printf("📊 FX rate created: %s/%s = %.6f on %s", fromCurrency, toCurrency, rate, rateDate.Format("2006-01-02"))
	return fxRate, nil
}

// GetFXRate retrieves the rate for a currency pair on a specific date
func (a *App) GetFXRate(fromCurrency, toCurrency string, date time.Time) (*FXRate, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var rate FXRate
	// Get exact date or most recent prior date
	err := a.db.Where("from_currency = ? AND to_currency = ? AND rate_date <= ?",
		fromCurrency, toCurrency, date).
		Order("rate_date DESC").
		First(&rate).Error

	if err != nil {
		return nil, fmt.Errorf("no rate found for %s/%s on or before %s", fromCurrency, toCurrency, date.Format("2006-01-02"))
	}

	return &rate, nil
}

// GetLatestFXRate retrieves the most recent rate for a currency pair
func (a *App) GetLatestFXRate(fromCurrency, toCurrency string) (*FXRate, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var rate FXRate
	err := a.db.Where("from_currency = ? AND to_currency = ?", fromCurrency, toCurrency).
		Order("rate_date DESC").
		First(&rate).Error

	if err != nil {
		return nil, fmt.Errorf("no rate found for %s/%s", fromCurrency, toCurrency)
	}

	return &rate, nil
}

// GetFXRateHistory retrieves rate history for a currency pair
func (a *App) GetFXRateHistory(fromCurrency, toCurrency string, startDate, endDate time.Time) ([]FXRate, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var rates []FXRate
	err := a.db.Where("from_currency = ? AND to_currency = ? AND rate_date BETWEEN ? AND ?",
		fromCurrency, toCurrency, startDate, endDate).
		Order("rate_date ASC").
		Find(&rates).Error

	return rates, err
}

// GetAllFXRates retrieves all rates for a base currency
func (a *App) GetAllFXRates(baseCurrency string, date time.Time) ([]FXRate, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var rates []FXRate
	// Get latest rates for each currency pair up to the given date
	subquery := a.db.Model(&FXRate{}).
		Select("from_currency, to_currency, MAX(rate_date) as max_date").
		Where("(from_currency = ? OR to_currency = ?) AND rate_date <= ?", baseCurrency, baseCurrency, date).
		Group("from_currency, to_currency")

	err := a.db.Where("(from_currency, to_currency, rate_date) IN (?)", subquery).
		Find(&rates).Error

	return rates, err
}

// DeleteFXRate removes a rate entry
func (a *App) DeleteFXRate(rateID string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	return a.db.Delete(&FXRate{}, "id = ?", rateID).Error
}

// =============================================================================
// FX REVALUATION CALCULATIONS
// =============================================================================

// CalculateFXRevaluation computes unrealized gain/loss for a foreign currency account
func (a *App) CalculateFXRevaluation(bankAccountID string, revaluationDate time.Time) (*FXRevaluation, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Wrap the entire calculation in a transaction so the account's BookingRate cannot
	// be modified between the account read and the revaluation insert.
	var reval *FXRevaluation
	txErr := a.db.Transaction(func(tx *gorm.DB) error {
		// Get bank account details (inside transaction — consistent snapshot)
		var account CompanyBankAccount
		if err := tx.First(&account, "id = ?", bankAccountID).Error; err != nil {
			return fmt.Errorf("bank account not found: %w", err)
		}

		if account.Currency == "BHD" {
			return fmt.Errorf("BHD accounts do not require FX revaluation")
		}

		// Get current balance from latest statement
		var latestStatement BankStatement
		if err := tx.Where("bank_account_id = ?", bankAccountID).
			Order("period_end DESC").
			First(&latestStatement).Error; err != nil {
			return fmt.Errorf("no bank statement found for account")
		}

		foreignBalance := latestStatement.ClosingBalance

		// Get previous revaluation (if any)
		var prevReval FXRevaluation
		var previousRate float64 = 0.0
		var previousBHD float64 = 0.0

		if err := tx.Where("bank_account_id = ?", bankAccountID).
			Order("revaluation_date DESC").
			First(&prevReval).Error; err == nil {
			previousRate = prevReval.CurrentRate
			previousBHD = prevReval.CurrentBHD
		} else {
			// First revaluation: use the account's booking rate (set when account was opened).
			// Assuming 1.0 produced wildly incorrect gain/loss figures for foreign accounts.
			if account.BookingRate > 0 {
				previousRate = account.BookingRate
				previousBHD = foreignBalance * account.BookingRate
			} else {
				// No prior revaluation and booking rate not recorded.
				// Fall back to current rate as baseline so gain/loss = 0 on first run (safe).
				// Users should set BookingRate on the account for accurate historical comparison.
				log.Printf("⚠️ WARNING: No booking_rate set for %s account %s — using current rate as baseline (first revaluation will show zero gain/loss)",
					account.Currency, account.AccountNumber)
				currentFXRateForBaseline, baseErr := a.GetFXRate(account.Currency, "BHD", revaluationDate)
				if baseErr != nil {
					return fmt.Errorf("cannot revalue %s account %s: no prior revaluation, no booking rate, and no current FX rate: %w",
						account.Currency, account.AccountNumber, baseErr)
				}
				previousRate = currentFXRateForBaseline.Rate
				previousBHD = foreignBalance * previousRate
			}
		}

		// Get current FX rate
		currentFXRate, err := a.GetFXRate(account.Currency, "BHD", revaluationDate)
		if err != nil {
			return fmt.Errorf("no FX rate available for %s/BHD: %w", account.Currency, err)
		}

		// Calculate current BHD value and gain/loss
		currentBHD := foreignBalance * currentFXRate.Rate
		gainLoss := currentBHD - previousBHD

		r := &FXRevaluation{
			BankAccountID:   bankAccountID,
			RevaluationDate: revaluationDate,
			ForeignCurrency: account.Currency,
			ForeignBalance:  foreignBalance,
			PreviousRate:    previousRate,
			PreviousBHD:     previousBHD,
			CurrentRate:     currentFXRate.Rate,
			CurrentBHD:      currentBHD,
			GainLossBHD:     gainLoss,
			IsPosted:        false,
		}

		if err := tx.Create(r).Error; err != nil {
			return fmt.Errorf("failed to create revaluation: %w", err)
		}

		gainLossType := "GAIN"
		if gainLoss < 0 {
			gainLossType = "LOSS"
		}
		log.Printf("💱 FX revaluation: %s %.3f %s → %.3f BHD (%s: %.3f BHD)",
			account.AccountNumber, foreignBalance, account.Currency, currentBHD, gainLossType, gainLoss)

		reval = r
		return nil
	})

	return reval, txErr
}

// GetFXRevaluations retrieves revaluation history for an account
func (a *App) GetFXRevaluations(bankAccountID string) ([]FXRevaluation, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var revaluations []FXRevaluation
	err := a.db.Where("bank_account_id = ?", bankAccountID).
		Order("revaluation_date DESC").
		Find(&revaluations).Error

	return revaluations, err
}

// GetUnpostedRevaluations retrieves all unposted revaluations
func (a *App) GetUnpostedRevaluations() ([]FXRevaluation, float64, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, 0, err
	}
	if a.db == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	var revaluations []FXRevaluation
	err := a.db.Where("is_posted = ?", false).
		Order("revaluation_date ASC").
		Find(&revaluations).Error

	if err != nil {
		return nil, 0, err
	}

	var totalGainLoss float64
	for _, r := range revaluations {
		totalGainLoss += r.GainLossBHD
	}

	return revaluations, totalGainLoss, nil
}

// PostFXRevaluation posts a revaluation to the general ledger
func (a *App) PostFXRevaluation(revaluationID, user string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var reval FXRevaluation
	if err := a.db.First(&reval, "id = ?", revaluationID).Error; err != nil {
		return fmt.Errorf("revaluation not found: %w", err)
	}

	if reval.IsPosted {
		return fmt.Errorf("revaluation already posted")
	}

	// In a full system, this would create a journal entry
	// For now, we just mark it as posted
	now := time.Now()
	result := a.db.Model(&reval).Updates(map[string]interface{}{
		"is_posted": true,
		"posted_by": user,
		"posted_at": now,
	})

	if result.Error != nil {
		return result.Error
	}

	log.Printf("📝 FX revaluation posted: %.3f BHD gain/loss by %s", reval.GainLossBHD, user)
	return nil
}

// ReverseRevaluation reverses a posted revaluation
func (a *App) ReverseRevaluation(revaluationID, user, reason string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var reval FXRevaluation
	if err := a.db.First(&reval, "id = ?", revaluationID).Error; err != nil {
		return fmt.Errorf("revaluation not found: %w", err)
	}

	if !reval.IsPosted {
		// Just delete if not posted
		return a.db.Delete(&reval).Error
	}

	// Create reversing entry
	reversing := &FXRevaluation{
		BankAccountID:   reval.BankAccountID,
		RevaluationDate: time.Now(),
		ForeignCurrency: reval.ForeignCurrency,
		ForeignBalance:  reval.ForeignBalance,
		PreviousRate:    reval.CurrentRate,
		PreviousBHD:     reval.CurrentBHD,
		CurrentRate:     reval.PreviousRate,
		CurrentBHD:      reval.PreviousBHD,
		GainLossBHD:     -reval.GainLossBHD, // Reverse the gain/loss
		IsPosted:        true,
		PostedBy:        user,
	}
	now := time.Now()
	reversing.PostedAt = &now

	if err := a.db.Create(reversing).Error; err != nil {
		return fmt.Errorf("failed to create reversing entry: %w", err)
	}

	log.Printf("🔄 FX revaluation reversed: %.3f BHD by %s - %s", reval.GainLossBHD, user, reason)
	return nil
}

// =============================================================================
// BATCH REVALUATION
// =============================================================================

// RevalueAllForeignAccounts performs revaluation for all foreign currency accounts
func (a *App) RevalueAllForeignAccounts(revaluationDate time.Time) ([]FXRevaluation, float64, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, 0, err
	}
	if a.db == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	// Get all foreign currency accounts
	var accounts []CompanyBankAccount
	if err := a.db.Where("currency != ?", "BHD").Find(&accounts).Error; err != nil {
		return nil, 0, err
	}

	var revaluations []FXRevaluation
	var totalGainLoss float64

	for _, account := range accounts {
		reval, err := a.CalculateFXRevaluation(account.ID, revaluationDate)
		if err != nil {
			log.Printf("⚠️ Failed to revalue %s: %v", account.AccountNumber, err)
			continue
		}
		revaluations = append(revaluations, *reval)
		totalGainLoss += reval.GainLossBHD
	}

	gainLossType := "NET GAIN"
	if totalGainLoss < 0 {
		gainLossType = "NET LOSS"
	}
	log.Printf("💱 Batch revaluation complete: %d accounts, %s: %.3f BHD", len(revaluations), gainLossType, totalGainLoss)

	return revaluations, totalGainLoss, nil
}

// =============================================================================
// FX REPORTS
// =============================================================================

// FXExposureReport provides a summary of foreign currency exposure
type FXExposureReport struct {
	Currency        string  `json:"currency"`
	AccountCount    int     `json:"account_count"`
	TotalForeign    float64 `json:"total_foreign"`
	CurrentRate     float64 `json:"current_rate"`
	TotalBHD        float64 `json:"total_bhd"`
	UnrealizedGain  float64 `json:"unrealized_gain"`
	PercentExposure float64 `json:"percent_exposure"`
}

// GetFXExposureReport generates a foreign currency exposure report
func (a *App) GetFXExposureReport() ([]FXExposureReport, float64, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, 0, err
	}
	if a.db == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	type currencyGroup struct {
		Currency     string
		TotalBalance float64
		AccountCount int
	}

	var accounts []CompanyBankAccount
	if err := a.db.Where("UPPER(currency) != ? AND is_active = ?", "BHD", true).
		Order("currency ASC, display_order ASC, bank_name ASC").
		Find(&accounts).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to load foreign currency accounts: %w", err)
	}

	statuses := []string{"Imported", "InProgress", "In Progress", "Reconciled", "Verified"}
	groupMap := make(map[string]*currencyGroup)
	for _, account := range accounts {
		currency := strings.ToUpper(strings.TrimSpace(account.Currency))
		if currency == "" || currency == "BHD" {
			continue
		}
		group := groupMap[currency]
		if group == nil {
			group = &currencyGroup{Currency: currency}
			groupMap[currency] = group
		}
		group.AccountCount++

		var latest BankStatement
		err := a.db.Where("bank_account_id = ? AND status IN ?", account.ID, statuses).
			Order("period_end DESC").
			First(&latest).Error
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				return nil, 0, fmt.Errorf("failed to load latest statement for %s: %w", account.AccountNumber, err)
			}
			continue
		}
		group.TotalBalance += latest.ClosingBalance
	}

	groups := make([]currencyGroup, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, *group)
	}

	var reports []FXExposureReport
	var totalBHDExposure float64

	for _, g := range groups {
		rate, err := a.GetLatestFXRate(g.Currency, "BHD")
		rateValue := 0.0
		if err == nil {
			rateValue = rate.Rate
		}

		bhdValue := g.TotalBalance * rateValue

		// Get unrealized gain from latest revaluation
		var latestReval FXRevaluation
		var unrealizedGain float64
		a.db.Where("foreign_currency = ? AND is_posted = ?", g.Currency, false).
			Order("revaluation_date DESC").
			First(&latestReval)
		if latestReval.ID != "" {
			unrealizedGain = latestReval.GainLossBHD
		}

		reports = append(reports, FXExposureReport{
			Currency:       g.Currency,
			AccountCount:   g.AccountCount,
			TotalForeign:   g.TotalBalance,
			CurrentRate:    rateValue,
			TotalBHD:       bhdValue,
			UnrealizedGain: unrealizedGain,
		})

		totalBHDExposure += bhdValue
	}

	// Calculate percentages
	for i := range reports {
		if totalBHDExposure > 0 {
			reports[i].PercentExposure = (reports[i].TotalBHD / totalBHDExposure) * 100
		}
	}

	return reports, totalBHDExposure, nil
}

// GetFXGainLossSummary provides YTD gain/loss summary
func (a *App) GetFXGainLossSummary(year int) (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	startDate := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC)

	var revaluations []FXRevaluation
	a.db.Where("revaluation_date BETWEEN ? AND ?", startDate, endDate).
		Order("revaluation_date ASC").
		Find(&revaluations)

	var totalGain, totalLoss, netGainLoss float64
	var postedGain, postedLoss float64

	for _, r := range revaluations {
		if r.GainLossBHD > 0 {
			totalGain += r.GainLossBHD
			if r.IsPosted {
				postedGain += r.GainLossBHD
			}
		} else {
			totalLoss += r.GainLossBHD
			if r.IsPosted {
				postedLoss += r.GainLossBHD
			}
		}
		netGainLoss += r.GainLossBHD
	}

	return map[string]interface{}{
		"year":              year,
		"total_gain":        totalGain,
		"total_loss":        totalLoss,
		"net_gain_loss":     netGainLoss,
		"posted_gain":       postedGain,
		"posted_loss":       postedLoss,
		"posted_net":        postedGain + postedLoss,
		"unposted_net":      netGainLoss - (postedGain + postedLoss),
		"revaluation_count": len(revaluations),
	}, nil
}
