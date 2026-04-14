package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type EmployeeCompensationProfile struct {
	Base
	EmployeeID         string     `gorm:"uniqueIndex;size:36" json:"employee_id"`
	Division           string     `gorm:"size:100;default:'PH Trading'" json:"division"`
	PayFrequency       string     `gorm:"size:20;default:'monthly'" json:"pay_frequency"`
	Currency           string     `gorm:"size:3;default:'BHD'" json:"currency"`
	BaseSalary         float64    `gorm:"type:decimal(15,3)" json:"base_salary"`
	HousingAllowance   float64    `gorm:"type:decimal(15,3)" json:"housing_allowance"`
	TransportAllowance float64    `gorm:"type:decimal(15,3)" json:"transport_allowance"`
	OtherAllowance     float64    `gorm:"type:decimal(15,3)" json:"other_allowance"`
	StandardDeduction  float64    `gorm:"type:decimal(15,3)" json:"standard_deduction"`
	TaxDeduction       float64    `gorm:"type:decimal(15,3)" json:"tax_deduction"`
	EmployerCost       float64    `gorm:"type:decimal(15,3)" json:"employer_cost"`
	EffectiveFrom      *time.Time `json:"effective_from"`
	EffectiveTo        *time.Time `json:"effective_to"`
	IsActive           bool       `gorm:"default:true;index" json:"is_active"`
	Notes              string     `gorm:"type:text" json:"notes"`

	EmployeeName string `gorm:"-" json:"employee_name,omitempty"`
	JobTitle     string `gorm:"-" json:"job_title,omitempty"`
}

func (EmployeeCompensationProfile) TableName() string { return "employee_compensation_profiles" }

type PayrollPeriod struct {
	Base
	Name        string     `gorm:"uniqueIndex;size:120" json:"name"`
	Division    string     `gorm:"size:100;default:'PH Trading'" json:"division"`
	PeriodStart time.Time  `gorm:"index" json:"period_start"`
	PeriodEnd   time.Time  `gorm:"index" json:"period_end"`
	PaymentDate *time.Time `gorm:"index" json:"payment_date"`
	Status      string     `gorm:"size:20;default:'open';index" json:"status"`
	Notes       string     `gorm:"type:text" json:"notes"`
}

func (PayrollPeriod) TableName() string { return "payroll_periods" }

type PayrollRun struct {
	Base
	RunNumber            string     `gorm:"uniqueIndex;size:50" json:"run_number"`
	PayrollPeriodID      string     `gorm:"index;size:36" json:"payroll_period_id"`
	Division             string     `gorm:"size:100;default:'PH Trading'" json:"division"`
	Status               string     `gorm:"size:20;default:'draft';index" json:"status"`
	GeneratedAt          *time.Time `json:"generated_at"`
	ApprovedAt           *time.Time `json:"approved_at"`
	ApprovedBy           string     `gorm:"size:36" json:"approved_by"`
	PostedAt             *time.Time `json:"posted_at"`
	PostedBy             string     `gorm:"size:36" json:"posted_by"`
	PaidAt               *time.Time `json:"paid_at"`
	PaymentReference     string     `gorm:"size:120" json:"payment_reference"`
	BankAccountID        *string    `gorm:"size:36" json:"bank_account_id"`
	JournalEntryID       *string    `gorm:"size:36" json:"journal_entry_id"`
	PayoutJournalEntryID *string    `gorm:"size:36" json:"payout_journal_entry_id"`
	TotalEmployees       int        `json:"total_employees"`
	GrossTotal           float64    `gorm:"type:decimal(15,3)" json:"gross_total"`
	DeductionsTotal      float64    `gorm:"type:decimal(15,3)" json:"deductions_total"`
	NetTotal             float64    `gorm:"type:decimal(15,3)" json:"net_total"`
	EmployerCostTotal    float64    `gorm:"type:decimal(15,3)" json:"employer_cost_total"`
	Currency             string     `gorm:"size:3;default:'BHD'" json:"currency"`
	Notes                string     `gorm:"type:text" json:"notes"`

	PeriodName string           `gorm:"-" json:"period_name,omitempty"`
	Items      []PayrollRunItem `gorm:"-" json:"items,omitempty"`
	Payouts    []PayrollPayout  `gorm:"-" json:"payouts,omitempty"`
}

func (PayrollRun) TableName() string { return "payroll_runs" }

type PayrollRunItem struct {
	Base
	PayrollRunID          string  `gorm:"index;size:36" json:"payroll_run_id"`
	EmployeeID            string  `gorm:"index;size:36" json:"employee_id"`
	CompensationProfileID *string `gorm:"size:36" json:"compensation_profile_id"`
	EmployeeNameSnapshot  string  `gorm:"size:255" json:"employee_name_snapshot"`
	JobTitleSnapshot      string  `gorm:"size:120" json:"job_title_snapshot"`
	BaseSalary            float64 `gorm:"type:decimal(15,3)" json:"base_salary"`
	AllowancesTotal       float64 `gorm:"type:decimal(15,3)" json:"allowances_total"`
	DeductionsTotal       float64 `gorm:"type:decimal(15,3)" json:"deductions_total"`
	EmployerCostTotal     float64 `gorm:"type:decimal(15,3)" json:"employer_cost_total"`
	GrossPay              float64 `gorm:"type:decimal(15,3)" json:"gross_pay"`
	NetPay                float64 `gorm:"type:decimal(15,3)" json:"net_pay"`
	Status                string  `gorm:"size:20;default:'draft';index" json:"status"`
	Notes                 string  `gorm:"type:text" json:"notes"`

	EmployeeName string             `gorm:"-" json:"employee_name,omitempty"`
	PayoutID     string             `gorm:"-" json:"payout_id,omitempty"`
	PayoutStatus string             `gorm:"-" json:"payout_status,omitempty"`
	PayoutPaidAt *time.Time         `gorm:"-" json:"payout_paid_at,omitempty"`
	Components   []PayrollComponent `gorm:"-" json:"components,omitempty"`
}

func (PayrollRunItem) TableName() string { return "payroll_run_items" }

type PayrollComponent struct {
	Base
	PayrollRunItemID string  `gorm:"index;size:36" json:"payroll_run_item_id"`
	ComponentType    string  `gorm:"size:20;index" json:"component_type"`
	Code             string  `gorm:"size:40" json:"code"`
	Name             string  `gorm:"size:120" json:"name"`
	Amount           float64 `gorm:"type:decimal(15,3)" json:"amount"`
}

func (PayrollComponent) TableName() string { return "payroll_components" }

type PayrollPayout struct {
	Base
	PayrollRunID        string     `gorm:"index;size:36" json:"payroll_run_id"`
	PayrollRunItemID    string     `gorm:"index;size:36" json:"payroll_run_item_id"`
	EmployeeID          string     `gorm:"index;size:36" json:"employee_id"`
	Division            string     `gorm:"size:100;default:'PH Trading'" json:"division"`
	ScheduledAt         *time.Time `gorm:"index" json:"scheduled_at"`
	PaidAt              *time.Time `gorm:"index" json:"paid_at"`
	Amount              float64    `gorm:"type:decimal(15,3)" json:"amount"`
	Currency            string     `gorm:"size:3;default:'BHD'" json:"currency"`
	Status              string     `gorm:"size:20;default:'scheduled';index" json:"status"`
	PaymentReference    string     `gorm:"size:120" json:"payment_reference"`
	BankAccountID       *string    `gorm:"size:36" json:"bank_account_id"`
	BankStatementLineID *string    `gorm:"size:36" json:"bank_statement_line_id"`

	EmployeeName string `gorm:"-" json:"employee_name,omitempty"`
	RunNumber    string `gorm:"-" json:"run_number,omitempty"`
}

func (PayrollPayout) TableName() string { return "payroll_payouts" }

type PayrollDashboardSummary struct {
	ActiveProfiles           int     `json:"active_profiles"`
	OpenPeriods              int     `json:"open_periods"`
	DraftRuns                int     `json:"draft_runs"`
	ApprovedUnpaidRuns       int     `json:"approved_unpaid_runs"`
	MonthToDateNetPayroll    float64 `json:"month_to_date_net_payroll"`
	UpcomingPayrollLiability float64 `json:"upcoming_payroll_liability"`
}

func (a *App) EnsurePayrollFoundation() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	models := []interface{}{
		&EmployeeCompensationProfile{},
		&PayrollPeriod{},
		&PayrollRun{},
		&PayrollRunItem{},
		&PayrollComponent{},
		&PayrollPayout{},
	}
	for _, model := range models {
		if err := a.db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}
	a.addColumnIfNotExists("employee_compensation_profiles", "division", "TEXT DEFAULT 'PH Trading'")
	a.addColumnIfNotExists("payroll_periods", "division", "TEXT DEFAULT 'PH Trading'")
	a.addColumnIfNotExists("payroll_runs", "division", "TEXT DEFAULT 'PH Trading'")
	a.addColumnIfNotExists("payroll_payouts", "division", "TEXT DEFAULT 'PH Trading'")

	return nil
}

func (a *App) requirePayrollView() error {
	if err := a.requirePermission("payroll:view"); err != nil {
		return a.requirePermission("finance:view")
	}
	return nil
}

func (a *App) requirePayrollCreate() error {
	if err := a.requirePermission("payroll:create"); err != nil {
		return a.requirePermission("finance:create")
	}
	return nil
}

func (a *App) requirePayrollUpdate() error {
	if err := a.requirePermission("payroll:update"); err != nil {
		return a.requirePermission("finance:update")
	}
	return nil
}

func (a *App) requirePayrollApprove() error {
	if err := a.requirePermission("payroll:approve"); err != nil {
		return a.requirePermission("finance:update")
	}
	return nil
}

func (a *App) ListEmployeeCompensationProfiles(activeOnly bool) ([]EmployeeCompensationProfile, error) {
	if err := a.requirePayrollView(); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := a.db.Order("updated_at DESC")
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	var profiles []EmployeeCompensationProfile
	if err := query.Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("failed to list compensation profiles: %w", err)
	}
	return a.decorateCompensationProfiles(profiles), nil
}

func (a *App) UpsertEmployeeCompensationProfile(profile EmployeeCompensationProfile) (EmployeeCompensationProfile, error) {
	if strings.TrimSpace(profile.ID) == "" {
		if err := a.requirePayrollCreate(); err != nil {
			return EmployeeCompensationProfile{}, err
		}
	} else {
		if err := a.requirePayrollUpdate(); err != nil {
			return EmployeeCompensationProfile{}, err
		}
	}
	if a.db == nil {
		return EmployeeCompensationProfile{}, fmt.Errorf("database not initialized")
	}

	profile.EmployeeID = strings.TrimSpace(profile.EmployeeID)
	if profile.EmployeeID == "" {
		return EmployeeCompensationProfile{}, fmt.Errorf("employee is required")
	}

	var employee Employee
	if err := a.db.First(&employee, "id = ?", profile.EmployeeID).Error; err != nil {
		return EmployeeCompensationProfile{}, fmt.Errorf("employee not found: %w", err)
	}

	profile.PayFrequency = strings.ToLower(strings.TrimSpace(profile.PayFrequency))
	if profile.PayFrequency == "" {
		profile.PayFrequency = "monthly"
	}
	profile.Division = normalizeDivisionName(profile.Division)
	profile.Currency = strings.ToUpper(strings.TrimSpace(profile.Currency))
	if profile.Currency == "" {
		profile.Currency = "BHD"
	}

	profile.BaseSalary = maxFinanceAmount(profile.BaseSalary)
	profile.HousingAllowance = maxFinanceAmount(profile.HousingAllowance)
	profile.TransportAllowance = maxFinanceAmount(profile.TransportAllowance)
	profile.OtherAllowance = maxFinanceAmount(profile.OtherAllowance)
	profile.StandardDeduction = maxFinanceAmount(profile.StandardDeduction)
	profile.TaxDeduction = maxFinanceAmount(profile.TaxDeduction)
	profile.EmployerCost = maxFinanceAmount(profile.EmployerCost)
	profile.Notes = strings.TrimSpace(profile.Notes)

	var existing EmployeeCompensationProfile
	err := a.db.Where("employee_id = ?", profile.EmployeeID).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return EmployeeCompensationProfile{}, fmt.Errorf("failed to lookup compensation profile: %w", err)
	}

	now := time.Now()
	if err == gorm.ErrRecordNotFound {
		profile.CreatedBy = a.getCurrentUserID()
		if !profile.IsActive {
			profile.IsActive = true
		}
		if err := a.db.Create(&profile).Error; err != nil {
			return EmployeeCompensationProfile{}, fmt.Errorf("failed to create compensation profile: %w", err)
		}
		a.emitPayrollEvent("payroll:updated", map[string]interface{}{"entity": "compensation_profile", "action": "create", "id": profile.ID})
		return a.decorateCompensationProfile(profile), nil
	}

	updates := map[string]interface{}{
		"pay_frequency":       profile.PayFrequency,
		"division":            profile.Division,
		"currency":            profile.Currency,
		"base_salary":         profile.BaseSalary,
		"housing_allowance":   profile.HousingAllowance,
		"transport_allowance": profile.TransportAllowance,
		"other_allowance":     profile.OtherAllowance,
		"standard_deduction":  profile.StandardDeduction,
		"tax_deduction":       profile.TaxDeduction,
		"employer_cost":       profile.EmployerCost,
		"effective_from":      profile.EffectiveFrom,
		"effective_to":        profile.EffectiveTo,
		"is_active":           profile.IsActive,
		"notes":               profile.Notes,
		"updated_at":          now,
	}
	if err := a.db.Model(&existing).Updates(updates).Error; err != nil {
		return EmployeeCompensationProfile{}, fmt.Errorf("failed to update compensation profile: %w", err)
	}

	existing.PayFrequency = profile.PayFrequency
	existing.Division = profile.Division
	existing.Currency = profile.Currency
	existing.BaseSalary = profile.BaseSalary
	existing.HousingAllowance = profile.HousingAllowance
	existing.TransportAllowance = profile.TransportAllowance
	existing.OtherAllowance = profile.OtherAllowance
	existing.StandardDeduction = profile.StandardDeduction
	existing.TaxDeduction = profile.TaxDeduction
	existing.EmployerCost = profile.EmployerCost
	existing.EffectiveFrom = profile.EffectiveFrom
	existing.EffectiveTo = profile.EffectiveTo
	existing.IsActive = profile.IsActive
	existing.Notes = profile.Notes

	a.emitPayrollEvent("payroll:updated", map[string]interface{}{"entity": "compensation_profile", "action": "update", "id": existing.ID})
	return a.decorateCompensationProfile(existing), nil
}

func (a *App) ListPayrollPeriods(includeClosed bool) ([]PayrollPeriod, error) {
	if err := a.requirePayrollView(); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := a.db.Order("period_start DESC")
	if !includeClosed {
		query = query.Where("status != ?", "closed")
	}

	var periods []PayrollPeriod
	if err := query.Find(&periods).Error; err != nil {
		return nil, fmt.Errorf("failed to list payroll periods: %w", err)
	}
	return periods, nil
}

func (a *App) CreatePayrollPeriod(period PayrollPeriod) (PayrollPeriod, error) {
	if err := a.requirePayrollCreate(); err != nil {
		return PayrollPeriod{}, err
	}
	if a.db == nil {
		return PayrollPeriod{}, fmt.Errorf("database not initialized")
	}

	if period.PeriodStart.IsZero() || period.PeriodEnd.IsZero() {
		return PayrollPeriod{}, fmt.Errorf("period start and end are required")
	}
	if period.PeriodEnd.Before(period.PeriodStart) {
		return PayrollPeriod{}, fmt.Errorf("period end must be on or after period start")
	}

	period.Name = strings.TrimSpace(period.Name)
	period.Division = normalizeDivisionName(period.Division)
	if period.Name == "" {
		period.Name = fmt.Sprintf("%s Payroll - %s", period.PeriodStart.Format("Jan 2006"), period.Division)
	}
	if period.PaymentDate == nil {
		paymentDate := period.PeriodEnd
		period.PaymentDate = &paymentDate
	}
	period.Status = strings.ToLower(strings.TrimSpace(period.Status))
	if period.Status == "" {
		period.Status = "open"
	}
	period.Notes = strings.TrimSpace(period.Notes)
	period.CreatedBy = a.getCurrentUserID()

	var count int64
	if err := a.db.Model(&PayrollPeriod{}).Where("name = ?", period.Name).Count(&count).Error; err == nil && count > 0 {
		return PayrollPeriod{}, fmt.Errorf("payroll period %q already exists", period.Name)
	}

	if err := a.db.Create(&period).Error; err != nil {
		return PayrollPeriod{}, fmt.Errorf("failed to create payroll period: %w", err)
	}
	a.emitPayrollEvent("payroll:updated", map[string]interface{}{"entity": "payroll_period", "action": "create", "id": period.ID})
	return period, nil
}

func (a *App) ListPayrollRuns(payrollPeriodID string) ([]PayrollRun, error) {
	if err := a.requirePayrollView(); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := a.db.Order("created_at DESC")
	if strings.TrimSpace(payrollPeriodID) != "" {
		query = query.Where("payroll_period_id = ?", payrollPeriodID)
	}

	var runs []PayrollRun
	if err := query.Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("failed to list payroll runs: %w", err)
	}
	return a.decoratePayrollRuns(runs), nil
}

func (a *App) GetPayrollRun(runID string) (PayrollRun, error) {
	if err := a.requirePayrollView(); err != nil {
		return PayrollRun{}, err
	}
	if a.db == nil {
		return PayrollRun{}, fmt.Errorf("database not initialized")
	}

	runID = strings.TrimSpace(runID)
	if runID == "" {
		return PayrollRun{}, fmt.Errorf("payroll run id is required")
	}

	var run PayrollRun
	if err := a.db.First(&run, "id = ?", runID).Error; err != nil {
		return PayrollRun{}, fmt.Errorf("payroll run not found: %w", err)
	}

	var items []PayrollRunItem
	if err := a.db.Where("payroll_run_id = ?", run.ID).Order("employee_name_snapshot ASC").Find(&items).Error; err != nil {
		return PayrollRun{}, fmt.Errorf("failed to load payroll run items: %w", err)
	}

	itemIDs := make([]string, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}

	var components []PayrollComponent
	if len(itemIDs) > 0 {
		if err := a.db.Where("payroll_run_item_id IN ?", itemIDs).Order("component_type ASC, name ASC").Find(&components).Error; err != nil {
			return PayrollRun{}, fmt.Errorf("failed to load payroll components: %w", err)
		}
	}

	componentMap := map[string][]PayrollComponent{}
	for _, component := range components {
		componentMap[component.PayrollRunItemID] = append(componentMap[component.PayrollRunItemID], component)
	}

	payouts, err := a.ListPayrollPayouts(run.ID)
	if err != nil {
		return PayrollRun{}, err
	}
	payoutMap := map[string]PayrollPayout{}
	for _, payout := range payouts {
		payoutMap[payout.PayrollRunItemID] = payout
	}

	items = a.decoratePayrollRunItems(items)
	for i := range items {
		items[i].Components = componentMap[items[i].ID]
		if payout, ok := payoutMap[items[i].ID]; ok {
			items[i].PayoutID = payout.ID
			items[i].PayoutStatus = payout.Status
			items[i].PayoutPaidAt = payout.PaidAt
		}
	}

	run = a.decoratePayrollRun(run)
	run.Items = items
	run.Payouts = payouts
	return run, nil
}

func (a *App) GeneratePayrollRun(payrollPeriodID string) (PayrollRun, error) {
	if err := a.requirePayrollCreate(); err != nil {
		return PayrollRun{}, err
	}
	if a.db == nil {
		return PayrollRun{}, fmt.Errorf("database not initialized")
	}

	payrollPeriodID = strings.TrimSpace(payrollPeriodID)
	if payrollPeriodID == "" {
		return PayrollRun{}, fmt.Errorf("payroll period is required")
	}

	var period PayrollPeriod
	if err := a.db.First(&period, "id = ?", payrollPeriodID).Error; err != nil {
		return PayrollRun{}, fmt.Errorf("payroll period not found: %w", err)
	}

	var profiles []EmployeeCompensationProfile
	if err := a.db.Where(
		"is_active = ? AND division = ? AND (effective_from IS NULL OR effective_from <= ?) AND (effective_to IS NULL OR effective_to >= ?)",
		true, normalizeDivisionName(period.Division), period.PeriodEnd, period.PeriodStart,
	).Find(&profiles).Error; err != nil {
		return PayrollRun{}, fmt.Errorf("failed to load compensation profiles: %w", err)
	}
	if len(profiles) == 0 {
		return PayrollRun{}, fmt.Errorf("no active compensation profiles available for this payroll period")
	}

	employeeIDs := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		employeeIDs = append(employeeIDs, profile.EmployeeID)
	}
	employees := a.employeeLookupMap(employeeIDs)

	var run PayrollRun
	existingErr := a.db.Where("payroll_period_id = ?", payrollPeriodID).Order("created_at DESC").First(&run).Error
	if existingErr != nil && existingErr != gorm.ErrRecordNotFound {
		return PayrollRun{}, fmt.Errorf("failed to lookup payroll run: %w", existingErr)
	}
	if existingErr == nil && run.Status != "draft" {
		return PayrollRun{}, fmt.Errorf("payroll run already exists for this period and is %s", run.Status)
	}

	now := time.Now()
	items := make([]PayrollRunItem, 0, len(profiles))
	components := make([]PayrollComponent, 0, len(profiles)*6)
	payouts := make([]PayrollPayout, 0, len(profiles))

	var grossTotal, deductionsTotal, netTotal, employerCostTotal float64
	totalEmployees := 0
	paymentDate := period.PeriodEnd
	if period.PaymentDate != nil {
		paymentDate = *period.PaymentDate
	}

	for _, profile := range profiles {
		employee, ok := employees[profile.EmployeeID]
		if !ok || !employee.IsActive {
			continue
		}

		allowances := profile.HousingAllowance + profile.TransportAllowance + profile.OtherAllowance
		deductions := profile.StandardDeduction + profile.TaxDeduction
		gross := profile.BaseSalary + allowances
		net := gross - deductions
		if net < 0 {
			net = 0
		}
		employerCost := maxFinanceAmount(profile.EmployerCost)

		item := PayrollRunItem{
			Base:                  Base{ID: uuid.New().String(), CreatedBy: a.getCurrentUserID(), CreatedAt: now, UpdatedAt: now},
			PayrollRunID:          run.ID,
			EmployeeID:            profile.EmployeeID,
			CompensationProfileID: &profile.ID,
			EmployeeNameSnapshot:  employee.FullName,
			JobTitleSnapshot:      employee.JobTitle,
			BaseSalary:            profile.BaseSalary,
			AllowancesTotal:       allowances,
			DeductionsTotal:       deductions,
			EmployerCostTotal:     employerCost,
			GrossPay:              gross,
			NetPay:                net,
			Status:                "draft",
		}
		item.Components = buildPayrollComponents(item.ID, profile, now, a.getCurrentUserID())
		items = append(items, item)
		components = append(components, item.Components...)
		payouts = append(payouts, PayrollPayout{
			Base:             Base{ID: uuid.New().String(), CreatedBy: a.getCurrentUserID(), CreatedAt: now, UpdatedAt: now},
			PayrollRunID:     run.ID,
			PayrollRunItemID: item.ID,
			EmployeeID:       item.EmployeeID,
			Division:         normalizeDivisionName(period.Division),
			ScheduledAt:      &paymentDate,
			Amount:           net,
			Currency:         profile.Currency,
			Status:           "scheduled",
		})

		grossTotal += gross
		deductionsTotal += deductions
		netTotal += net
		employerCostTotal += employerCost
		totalEmployees++
	}

	if totalEmployees == 0 {
		return PayrollRun{}, fmt.Errorf("no active employees available for payroll generation")
	}

	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if existingErr == gorm.ErrRecordNotFound {
		run = PayrollRun{
			Base:              Base{CreatedBy: a.getCurrentUserID(), CreatedAt: now, UpdatedAt: now},
			RunNumber:         generatePayrollRunNumber(period, now),
			PayrollPeriodID:   period.ID,
			Division:          normalizeDivisionName(period.Division),
			Status:            "draft",
			GeneratedAt:       &now,
			TotalEmployees:    totalEmployees,
			GrossTotal:        grossTotal,
			DeductionsTotal:   deductionsTotal,
			NetTotal:          netTotal,
			EmployerCostTotal: employerCostTotal,
			Currency:          "BHD",
		}
		if err := tx.Create(&run).Error; err != nil {
			tx.Rollback()
			return PayrollRun{}, fmt.Errorf("failed to create payroll run: %w", err)
		}
	} else {
		var existingItems []PayrollRunItem
		if err := tx.Where("payroll_run_id = ?", run.ID).Find(&existingItems).Error; err != nil {
			tx.Rollback()
			return PayrollRun{}, fmt.Errorf("failed to load existing payroll items: %w", err)
		}
		if len(existingItems) > 0 {
			oldItemIDs := make([]string, 0, len(existingItems))
			for _, item := range existingItems {
				oldItemIDs = append(oldItemIDs, item.ID)
			}
			if err := tx.Where("payroll_run_item_id IN ?", oldItemIDs).Delete(&PayrollComponent{}).Error; err != nil {
				tx.Rollback()
				return PayrollRun{}, fmt.Errorf("failed to clear payroll components: %w", err)
			}
		}
		if err := tx.Where("payroll_run_id = ?", run.ID).Delete(&PayrollPayout{}).Error; err != nil {
			tx.Rollback()
			return PayrollRun{}, fmt.Errorf("failed to clear payroll payouts: %w", err)
		}
		if err := tx.Where("payroll_run_id = ?", run.ID).Delete(&PayrollRunItem{}).Error; err != nil {
			tx.Rollback()
			return PayrollRun{}, fmt.Errorf("failed to clear payroll items: %w", err)
		}
		run.Status = "draft"
		run.Division = normalizeDivisionName(period.Division)
		run.GeneratedAt = &now
		run.ApprovedAt = nil
		run.ApprovedBy = ""
		run.PostedAt = nil
		run.PostedBy = ""
		run.PaidAt = nil
		run.PaymentReference = ""
		run.BankAccountID = nil
		run.JournalEntryID = nil
		run.PayoutJournalEntryID = nil
		run.TotalEmployees = totalEmployees
		run.GrossTotal = grossTotal
		run.DeductionsTotal = deductionsTotal
		run.NetTotal = netTotal
		run.EmployerCostTotal = employerCostTotal
		run.Currency = "BHD"
		if err := tx.Model(&run).Updates(map[string]interface{}{
			"status":                  run.Status,
			"division":                run.Division,
			"generated_at":            run.GeneratedAt,
			"approved_at":             nil,
			"approved_by":             "",
			"posted_at":               nil,
			"posted_by":               "",
			"paid_at":                 nil,
			"payment_reference":       "",
			"bank_account_id":         nil,
			"journal_entry_id":        nil,
			"payout_journal_entry_id": nil,
			"total_employees":         run.TotalEmployees,
			"gross_total":             run.GrossTotal,
			"deductions_total":        run.DeductionsTotal,
			"net_total":               run.NetTotal,
			"employer_cost_total":     run.EmployerCostTotal,
			"currency":                run.Currency,
			"updated_at":              now,
		}).Error; err != nil {
			tx.Rollback()
			return PayrollRun{}, fmt.Errorf("failed to update payroll run: %w", err)
		}
	}

	for i := range items {
		items[i].PayrollRunID = run.ID
	}
	for i := range payouts {
		payouts[i].PayrollRunID = run.ID
	}

	if err := tx.Create(&items).Error; err != nil {
		tx.Rollback()
		return PayrollRun{}, fmt.Errorf("failed to create payroll items: %w", err)
	}
	if len(components) > 0 {
		if err := tx.Create(&components).Error; err != nil {
			tx.Rollback()
			return PayrollRun{}, fmt.Errorf("failed to create payroll components: %w", err)
		}
	}
	if len(payouts) > 0 {
		if err := tx.Create(&payouts).Error; err != nil {
			tx.Rollback()
			return PayrollRun{}, fmt.Errorf("failed to create payroll payouts: %w", err)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return PayrollRun{}, fmt.Errorf("failed to commit payroll run: %w", err)
	}

	a.emitPayrollEvent("payroll:updated", map[string]interface{}{"entity": "payroll_run", "action": "generate", "id": run.ID})
	return a.GetPayrollRun(run.ID)
}

func (a *App) ApprovePayrollRun(runID, notes string) (PayrollRun, error) {
	if err := a.requirePayrollApprove(); err != nil {
		return PayrollRun{}, err
	}
	if a.db == nil {
		return PayrollRun{}, fmt.Errorf("database not initialized")
	}

	var run PayrollRun
	if err := a.db.First(&run, "id = ?", strings.TrimSpace(runID)).Error; err != nil {
		return PayrollRun{}, fmt.Errorf("payroll run not found: %w", err)
	}
	if run.Status != "draft" {
		return PayrollRun{}, fmt.Errorf("only draft payroll runs can be approved")
	}

	now := time.Now()
	notes = strings.TrimSpace(notes)
	if err := a.db.Model(&run).Updates(map[string]interface{}{
		"status":      "approved",
		"approved_at": &now,
		"approved_by": a.getPayrollActorID(),
		"notes":       mergeNotes(run.Notes, notes),
		"updated_at":  now,
	}).Error; err != nil {
		return PayrollRun{}, fmt.Errorf("failed to approve payroll run: %w", err)
	}
	_ = a.db.Model(&PayrollRunItem{}).Where("payroll_run_id = ?", run.ID).Updates(map[string]interface{}{
		"status":     "approved",
		"updated_at": now,
	}).Error

	a.emitPayrollEvent("payroll:updated", map[string]interface{}{"entity": "payroll_run", "action": "approve", "id": run.ID})
	return a.GetPayrollRun(run.ID)
}

func (a *App) PostPayrollRun(runID string) (PayrollRun, error) {
	if err := a.requirePayrollUpdate(); err != nil {
		return PayrollRun{}, err
	}
	if a.db == nil {
		return PayrollRun{}, fmt.Errorf("database not initialized")
	}

	run, err := a.GetPayrollRun(strings.TrimSpace(runID))
	if err != nil {
		return PayrollRun{}, err
	}
	if run.Status == "posted" || run.Status == "paid" {
		return run, nil
	}
	if run.Status != "approved" {
		return PayrollRun{}, fmt.Errorf("payroll run must be approved before posting")
	}

	journalID, err := a.postPayrollAccrualJournal(&run)
	if err != nil {
		return PayrollRun{}, err
	}

	now := time.Now()
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&PayrollRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{
		"status":           "posted",
		"posted_at":        &now,
		"posted_by":        a.getPayrollActorID(),
		"journal_entry_id": journalID,
		"updated_at":       now,
	}).Error; err != nil {
		tx.Rollback()
		return PayrollRun{}, fmt.Errorf("failed to mark payroll run posted: %w", err)
	}
	if err := tx.Model(&PayrollRunItem{}).Where("payroll_run_id = ?", run.ID).Updates(map[string]interface{}{
		"status":     "posted",
		"updated_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return PayrollRun{}, fmt.Errorf("failed to update payroll run items: %w", err)
	}

	run.Status = "posted"
	run.PostedAt = &now
	run.PostedBy = a.getPayrollActorID()
	run.JournalEntryID = &journalID
	if err := a.syncPayrollRunExpenseEntry(tx, &run); err != nil {
		tx.Rollback()
		return PayrollRun{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return PayrollRun{}, fmt.Errorf("failed to commit payroll posting: %w", err)
	}

	a.emitPayrollEvent("payroll:updated", map[string]interface{}{"entity": "payroll_run", "action": "post", "id": run.ID})
	a.emitExpenseEvent("expenses:updated", map[string]interface{}{"entity": "entry", "action": "payroll_sync", "id": run.ID})
	return a.GetPayrollRun(run.ID)
}

func (a *App) MarkPayrollRunPaid(runID, paidAtISO, paymentReference, bankAccountID string) (PayrollRun, error) {
	if err := a.requirePayrollUpdate(); err != nil {
		return PayrollRun{}, err
	}
	if a.db == nil {
		return PayrollRun{}, fmt.Errorf("database not initialized")
	}

	run, err := a.GetPayrollRun(strings.TrimSpace(runID))
	if err != nil {
		return PayrollRun{}, err
	}
	if run.Status == "paid" {
		return run, nil
	}
	if run.Status != "posted" {
		run, err = a.PostPayrollRun(run.ID)
		if err != nil {
			return PayrollRun{}, err
		}
	}

	paidAt, err := parseOptionalISODateTime(paidAtISO)
	if err != nil {
		return PayrollRun{}, err
	}
	if paidAt == nil {
		now := time.Now()
		paidAt = &now
	}

	payoutJournalID, err := a.postPayrollPayoutJournal(&run, paymentReference)
	if err != nil {
		return PayrollRun{}, err
	}

	var bankAccountRef *string
	if trimmed := strings.TrimSpace(bankAccountID); trimmed != "" {
		bankDivision := a.resolveBankAccountDivision(trimmed)
		runDivision := normalizeDivisionName(run.Division)
		if bankDivision != runDivision {
			return PayrollRun{}, fmt.Errorf("payroll run belongs to %s but bank account belongs to %s", runDivision, bankDivision)
		}
		bankAccountRef = &trimmed
	}

	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&PayrollPayout{}).Where("payroll_run_id = ?", run.ID).Updates(map[string]interface{}{
		"status":            "paid",
		"paid_at":           paidAt,
		"payment_reference": strings.TrimSpace(paymentReference),
		"bank_account_id":   bankAccountRef,
		"division":          normalizeDivisionName(run.Division),
		"updated_at":        time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return PayrollRun{}, fmt.Errorf("failed to mark payroll payouts paid: %w", err)
	}

	if err := tx.Model(&PayrollRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{
		"status":                  "paid",
		"paid_at":                 paidAt,
		"payment_reference":       strings.TrimSpace(paymentReference),
		"bank_account_id":         bankAccountRef,
		"division":                normalizeDivisionName(run.Division),
		"payout_journal_entry_id": payoutJournalID,
		"updated_at":              time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return PayrollRun{}, fmt.Errorf("failed to update payroll run payment status: %w", err)
	}
	if err := tx.Model(&PayrollRunItem{}).Where("payroll_run_id = ?", run.ID).Updates(map[string]interface{}{
		"status":     "paid",
		"updated_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return PayrollRun{}, fmt.Errorf("failed to update payroll run items: %w", err)
	}

	run.Status = "paid"
	run.PaidAt = paidAt
	run.PaymentReference = strings.TrimSpace(paymentReference)
	run.BankAccountID = bankAccountRef
	if payoutJournalID != "" {
		run.PayoutJournalEntryID = &payoutJournalID
	} else {
		run.PayoutJournalEntryID = nil
	}
	if err := a.syncPayrollRunExpenseEntry(tx, &run); err != nil {
		tx.Rollback()
		return PayrollRun{}, err
	}

	if err := tx.Commit().Error; err != nil {
		return PayrollRun{}, fmt.Errorf("failed to commit payroll payment: %w", err)
	}

	a.emitPayrollEvent("payroll:updated", map[string]interface{}{"entity": "payroll_run", "action": "paid", "id": run.ID})
	a.emitExpenseEvent("expenses:updated", map[string]interface{}{"entity": "entry", "action": "payroll_sync", "id": run.ID})
	return a.GetPayrollRun(run.ID)
}

func (a *App) ListPayrollPayouts(payrollRunID string) ([]PayrollPayout, error) {
	if err := a.requirePayrollView(); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := a.db.Order("scheduled_at DESC, created_at DESC")
	if strings.TrimSpace(payrollRunID) != "" {
		query = query.Where("payroll_run_id = ?", payrollRunID)
	}

	var payouts []PayrollPayout
	if err := query.Find(&payouts).Error; err != nil {
		return nil, fmt.Errorf("failed to list payroll payouts: %w", err)
	}
	return a.decoratePayrollPayouts(payouts), nil
}

func (a *App) ListUnreconciledPayrollPayouts() ([]PayrollPayout, error) {
	if err := a.requirePayrollView(); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var payouts []PayrollPayout
	if err := a.db.Where("status = ? AND bank_statement_line_id IS NULL", "paid").
		Order("paid_at DESC, created_at DESC").
		Find(&payouts).Error; err != nil {
		return nil, fmt.Errorf("failed to list unreconciled payroll payouts: %w", err)
	}
	return a.decoratePayrollPayouts(payouts), nil
}

func (a *App) ListPayrollDashboardSummary() (PayrollDashboardSummary, error) {
	if err := a.requirePayrollView(); err != nil {
		return PayrollDashboardSummary{}, err
	}
	if a.db == nil {
		return PayrollDashboardSummary{}, fmt.Errorf("database not initialized")
	}

	summary := PayrollDashboardSummary{}
	var activeProfiles int64
	var openPeriods int64
	var draftRuns int64
	var approvedUnpaidRuns int64
	_ = a.db.Model(&EmployeeCompensationProfile{}).Where("is_active = ?", true).Count(&activeProfiles).Error
	_ = a.db.Model(&PayrollPeriod{}).Where("status = ?", "open").Count(&openPeriods).Error
	_ = a.db.Model(&PayrollRun{}).Where("status = ?", "draft").Count(&draftRuns).Error
	_ = a.db.Model(&PayrollRun{}).Where("status IN ?", []string{"approved", "posted"}).Count(&approvedUnpaidRuns).Error
	summary.ActiveProfiles = int(activeProfiles)
	summary.OpenPeriods = int(openPeriods)
	summary.DraftRuns = int(draftRuns)
	summary.ApprovedUnpaidRuns = int(approvedUnpaidRuns)

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	type aggregate struct {
		Total float64
	}
	var mtd aggregate
	_ = a.db.Model(&PayrollRun{}).Select("COALESCE(SUM(net_total), 0) AS total").Where("status = ? AND paid_at >= ?", "paid", startOfMonth).Scan(&mtd).Error
	summary.MonthToDateNetPayroll = mtd.Total

	var liabilities aggregate
	_ = a.db.Model(&PayrollRun{}).Select("COALESCE(SUM(net_total + deductions_total + employer_cost_total), 0) AS total").Where("status IN ?", []string{"approved", "posted"}).Scan(&liabilities).Error
	summary.UpcomingPayrollLiability = liabilities.Total

	return summary, nil
}

func (a *App) postPayrollAccrualJournal(run *PayrollRun) (string, error) {
	salaryExpense, err := a.ensureSupportingAccount("6000", "Salaries & Wages", "Expense")
	if err != nil {
		return "", err
	}
	overheadExpense, err := a.ensureSupportingAccount("6050", "Payroll Overheads", "Expense")
	if err != nil {
		return "", err
	}
	payrollPayable, err := a.ensureSupportingAccount("2210", "Payroll Payable", "Liability")
	if err != nil {
		return "", err
	}
	deductionPayable, err := a.ensureSupportingAccount("2211", "Payroll Deductions Payable", "Liability")
	if err != nil {
		return "", err
	}
	employerPayable, err := a.ensureSupportingAccount("2212", "Payroll Employer Liabilities", "Liability")
	if err != nil {
		return "", err
	}

	now := time.Now()
	entryDate := now
	var period PayrollPeriod
	if err := a.db.First(&period, "id = ?", run.PayrollPeriodID).Error; err == nil {
		entryDate = period.PeriodEnd
	}

	lines := []JournalLine{
		newJournalLine(salaryExpense, run.GrossTotal, 0, "Payroll gross accrual", now, a.getCurrentUserID()),
		newJournalLine(payrollPayable, 0, run.NetTotal, "Payroll net payable", now, a.getCurrentUserID()),
	}
	if run.EmployerCostTotal > 0 {
		lines = append(lines,
			newJournalLine(overheadExpense, run.EmployerCostTotal, 0, "Payroll employer cost accrual", now, a.getCurrentUserID()),
			newJournalLine(employerPayable, 0, run.EmployerCostTotal, "Payroll employer liabilities", now, a.getCurrentUserID()),
		)
	}
	if run.DeductionsTotal > 0 {
		lines = append(lines, newJournalLine(deductionPayable, 0, run.DeductionsTotal, "Payroll deductions withheld", now, a.getCurrentUserID()))
	}

	journal := JournalEntry{
		Base:            Base{ID: uuid.New().String(), CreatedBy: a.getCurrentUserID(), CreatedAt: now, UpdatedAt: now},
		EntryNumber:     fmt.Sprintf("PAY-JE-%d-%04d", now.Year(), now.UnixNano()%10000),
		EntryDate:       entryDate,
		Description:     fmt.Sprintf("Payroll accrual %s", run.RunNumber),
		DebitTotal:      run.GrossTotal + run.EmployerCostTotal,
		CreditTotal:     run.NetTotal + run.DeductionsTotal + run.EmployerCostTotal,
		IsPosted:        true,
		PostedAt:        &now,
		PostedBy:        a.getPayrollActorID(),
		FiscalYear:      entryDate.Year(),
		FiscalPeriod:    int(entryDate.Month()),
		SourceType:      "payroll_run",
		SourceID:        run.ID,
		IsAutoGenerated: true,
	}

	return a.persistPostedJournal(journal, lines)
}

func (a *App) postPayrollPayoutJournal(run *PayrollRun, paymentReference string) (string, error) {
	if run.NetTotal <= 0 {
		return "", nil
	}

	payrollPayable, err := a.ensureSupportingAccount("2210", "Payroll Payable", "Liability")
	if err != nil {
		return "", err
	}
	cashAccount, err := a.ensureSupportingAccount("1000", "Cash", "Asset")
	if err != nil {
		return "", err
	}

	now := time.Now()
	lines := []JournalLine{
		newJournalLine(payrollPayable, run.NetTotal, 0, "Payroll payout clearing", now, a.getCurrentUserID()),
		newJournalLine(cashAccount, 0, run.NetTotal, "Payroll cash disbursement", now, a.getCurrentUserID()),
	}
	journal := JournalEntry{
		Base:            Base{ID: uuid.New().String(), CreatedBy: a.getCurrentUserID(), CreatedAt: now, UpdatedAt: now},
		EntryNumber:     fmt.Sprintf("PAY-OUT-%d-%04d", now.Year(), now.UnixNano()%10000),
		EntryDate:       now,
		Description:     fmt.Sprintf("Payroll payout %s %s", run.RunNumber, strings.TrimSpace(paymentReference)),
		DebitTotal:      run.NetTotal,
		CreditTotal:     run.NetTotal,
		IsPosted:        true,
		PostedAt:        &now,
		PostedBy:        a.getPayrollActorID(),
		FiscalYear:      now.Year(),
		FiscalPeriod:    int(now.Month()),
		SourceType:      "payroll_payout",
		SourceID:        run.ID,
		IsAutoGenerated: true,
	}

	return a.persistPostedJournal(journal, lines)
}

func (a *App) syncPayrollRunExpenseEntry(tx *gorm.DB, run *PayrollRun) error {
	if tx == nil {
		tx = a.db
	}
	if tx == nil || run == nil || strings.TrimSpace(run.ID) == "" || run.NetTotal <= 0 {
		return nil
	}

	category, err := a.ensurePayrollExpenseCategory(tx)
	if err != nil {
		return fmt.Errorf("failed to ensure payroll expense category: %w", err)
	}

	expenseDate, dueDate := a.resolvePayrollExpenseDates(tx, run)
	status := "posted"
	paymentStatus := "unpaid"
	if strings.EqualFold(run.Status, "paid") {
		status = "paid"
		paymentStatus = "paid"
	}

	description := fmt.Sprintf("Salary payment for %s", strings.TrimSpace(run.RunNumber))
	notes := fmt.Sprintf(
		"Linked from payroll run %s. Gross %.3f BHD, deductions %.3f BHD, employer cost %.3f BHD. Expense ledger tracks the net salary payout for operational payment visibility.",
		strings.TrimSpace(run.RunNumber),
		run.GrossTotal,
		run.DeductionsTotal,
		run.EmployerCostTotal,
	)

	var existing ExpenseEntry
	err = tx.Where("source_type = ? AND source_ref_id = ?", "payroll", run.ID).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to look up payroll expense entry: %w", err)
	}

	paymentReference := strings.TrimSpace(run.PaymentReference)
	now := time.Now()
	if err == gorm.ErrRecordNotFound {
		entryNumber, genErr := a.generateExpenseEntryNumber()
		if genErr != nil {
			return genErr
		}
		sourceRefID := run.ID
		entry := ExpenseEntry{
			Base:             Base{ID: uuid.New().String(), CreatedBy: a.getCurrentUserID(), CreatedAt: now, UpdatedAt: now},
			EntryNumber:      entryNumber,
			ExpenseDate:      expenseDate,
			DueDate:          &dueDate,
			Description:      description,
			CategoryID:       category.ID,
			SourceType:       "payroll",
			SourceRefID:      &sourceRefID,
			CostCenter:       "Payroll",
			Currency:         firstPopulatedString(run.Currency, "BHD"),
			Division:         normalizeDivisionName(run.Division),
			Amount:           maxFinanceAmount(run.NetTotal),
			VATAmount:        0,
			TotalAmount:      maxFinanceAmount(run.NetTotal),
			Status:           status,
			PaymentStatus:    paymentStatus,
			PostedAt:         run.PostedAt,
			PostedBy:         run.PostedBy,
			PaidAt:           run.PaidAt,
			PaymentMethod:    "Payroll Transfer",
			PaymentReference: paymentReference,
			BankAccountID:    run.BankAccountID,
			JournalEntryID:   run.JournalEntryID,
			Notes:            notes,
		}
		if paymentStatus != "paid" {
			entry.PaymentMethod = ""
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("failed to create payroll expense entry: %w", err)
		}
		a.recordExpenseApproval(entry.ID, "payroll_sync", run.RunNumber)
		return nil
	}

	updates := map[string]interface{}{
		"expense_date":      expenseDate,
		"due_date":          &dueDate,
		"description":       description,
		"category_id":       category.ID,
		"cost_center":       "Payroll",
		"currency":          firstPopulatedString(run.Currency, "BHD"),
		"division":          normalizeDivisionName(run.Division),
		"amount":            maxFinanceAmount(run.NetTotal),
		"vat_amount":        0,
		"total_amount":      maxFinanceAmount(run.NetTotal),
		"status":            status,
		"payment_status":    paymentStatus,
		"posted_at":         run.PostedAt,
		"posted_by":         run.PostedBy,
		"paid_at":           run.PaidAt,
		"payment_reference": paymentReference,
		"bank_account_id":   run.BankAccountID,
		"journal_entry_id":  run.JournalEntryID,
		"notes":             notes,
		"updated_at":        now,
	}
	if paymentStatus == "paid" {
		updates["payment_method"] = "Payroll Transfer"
	} else {
		updates["payment_method"] = ""
	}
	if err := tx.Model(&existing).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update payroll expense entry: %w", err)
	}
	return nil
}

func (a *App) ensurePayrollExpenseCategory(tx *gorm.DB) (ExpenseCategory, error) {
	if tx == nil {
		tx = a.db
	}
	var category ExpenseCategory
	if err := tx.Where("UPPER(code) = ?", "PAYROLL").First(&category).Error; err == nil {
		return category, nil
	} else if err != gorm.ErrRecordNotFound {
		return ExpenseCategory{}, err
	}

	account, err := a.ensureSupportingAccount("6000", accountNameForCode("6000"), "Expense")
	if err != nil {
		return ExpenseCategory{}, err
	}
	accountID := account.ID
	category = ExpenseCategory{
		Base:        Base{CreatedBy: a.getCurrentUserID()},
		Name:        "Employee Salaries",
		Code:        "PAYROLL",
		Description: "Salaries, wages, and payroll overhead",
		GLAccountID: &accountID,
		IsActive:    true,
		SortOrder:   30,
	}
	if err := tx.Create(&category).Error; err != nil {
		return ExpenseCategory{}, err
	}
	return category, nil
}

func (a *App) resolvePayrollExpenseDates(tx *gorm.DB, run *PayrollRun) (time.Time, time.Time) {
	if run == nil {
		now := time.Now()
		return now, now
	}
	if run.PaidAt != nil {
		return *run.PaidAt, *run.PaidAt
	}
	if tx != nil {
		var period PayrollPeriod
		if err := tx.First(&period, "id = ?", run.PayrollPeriodID).Error; err == nil {
			if period.PaymentDate != nil {
				return *period.PaymentDate, *period.PaymentDate
			}
			return period.PeriodEnd, period.PeriodEnd
		}
	}
	if run.PostedAt != nil {
		return *run.PostedAt, *run.PostedAt
	}
	if run.GeneratedAt != nil {
		return *run.GeneratedAt, *run.GeneratedAt
	}
	now := time.Now()
	return now, now
}

func firstPopulatedString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (a *App) persistPostedJournal(journal JournalEntry, lines []JournalLine) (string, error) {
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&journal).Error; err != nil {
		tx.Rollback()
		return "", fmt.Errorf("failed to create payroll journal entry: %w", err)
	}
	for i := range lines {
		lines[i].EntryID = journal.ID
	}
	if err := tx.Create(&lines).Error; err != nil {
		tx.Rollback()
		return "", fmt.Errorf("failed to create payroll journal lines: %w", err)
	}
	for _, line := range lines {
		var account ChartOfAccount
		if err := tx.First(&account, "id = ?", line.AccountID).Error; err != nil {
			tx.Rollback()
			return "", fmt.Errorf("failed to load payroll posting account: %w", err)
		}
		change := line.Credit - line.Debit
		if account.AccountType == "Asset" || account.AccountType == "Expense" {
			change = line.Debit - line.Credit
		}
		if err := tx.Model(&account).Update("balance", gorm.Expr("balance + ?", change)).Error; err != nil {
			tx.Rollback()
			return "", fmt.Errorf("failed to update payroll account balance: %w", err)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return "", fmt.Errorf("failed to commit payroll journal: %w", err)
	}
	return journal.ID, nil
}

func (a *App) decorateCompensationProfiles(profiles []EmployeeCompensationProfile) []EmployeeCompensationProfile {
	if len(profiles) == 0 {
		return profiles
	}
	employeeIDs := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		employeeIDs = append(employeeIDs, profile.EmployeeID)
	}
	employees := a.employeeLookupMap(employeeIDs)
	for i := range profiles {
		if employee, ok := employees[profiles[i].EmployeeID]; ok {
			profiles[i].EmployeeName = employee.FullName
			profiles[i].JobTitle = employee.JobTitle
		}
	}
	return profiles
}

func (a *App) decorateCompensationProfile(profile EmployeeCompensationProfile) EmployeeCompensationProfile {
	rows := a.decorateCompensationProfiles([]EmployeeCompensationProfile{profile})
	if len(rows) == 0 {
		return profile
	}
	return rows[0]
}

func (a *App) decoratePayrollRuns(runs []PayrollRun) []PayrollRun {
	if len(runs) == 0 {
		return runs
	}
	periodIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		periodIDs = append(periodIDs, run.PayrollPeriodID)
	}
	periodNames := map[string]string{}
	var periods []PayrollPeriod
	if err := a.db.Select("id", "name").Where("id IN ?", periodIDs).Find(&periods).Error; err == nil {
		for _, period := range periods {
			periodNames[period.ID] = period.Name
		}
	}
	for i := range runs {
		runs[i].PeriodName = periodNames[runs[i].PayrollPeriodID]
	}
	return runs
}

func (a *App) decoratePayrollRun(run PayrollRun) PayrollRun {
	rows := a.decoratePayrollRuns([]PayrollRun{run})
	if len(rows) == 0 {
		return run
	}
	return rows[0]
}

func (a *App) decoratePayrollRunItems(items []PayrollRunItem) []PayrollRunItem {
	if len(items) == 0 {
		return items
	}
	employeeIDs := make([]string, 0, len(items))
	for _, item := range items {
		employeeIDs = append(employeeIDs, item.EmployeeID)
	}
	employees := a.employeeLookupMap(employeeIDs)
	for i := range items {
		if employee, ok := employees[items[i].EmployeeID]; ok {
			items[i].EmployeeName = employee.FullName
		}
		if items[i].EmployeeName == "" {
			items[i].EmployeeName = items[i].EmployeeNameSnapshot
		}
	}
	return items
}

func (a *App) decoratePayrollPayouts(payouts []PayrollPayout) []PayrollPayout {
	if len(payouts) == 0 {
		return payouts
	}
	employeeIDs := make([]string, 0, len(payouts))
	runIDs := make([]string, 0, len(payouts))
	for _, payout := range payouts {
		employeeIDs = append(employeeIDs, payout.EmployeeID)
		runIDs = append(runIDs, payout.PayrollRunID)
	}
	employees := a.employeeLookupMap(employeeIDs)
	runNumbers := map[string]string{}
	var runs []PayrollRun
	if err := a.db.Select("id", "run_number").Where("id IN ?", runIDs).Find(&runs).Error; err == nil {
		for _, run := range runs {
			runNumbers[run.ID] = run.RunNumber
		}
	}
	for i := range payouts {
		if employee, ok := employees[payouts[i].EmployeeID]; ok {
			payouts[i].EmployeeName = employee.FullName
		}
		payouts[i].RunNumber = runNumbers[payouts[i].PayrollRunID]
	}
	return payouts
}

func (a *App) employeeLookupMap(employeeIDs []string) map[string]Employee {
	lookup := map[string]Employee{}
	if len(employeeIDs) == 0 || a.db == nil {
		return lookup
	}
	var employees []Employee
	if err := a.db.Where("id IN ?", employeeIDs).Find(&employees).Error; err != nil {
		return lookup
	}
	for _, employee := range employees {
		lookup[employee.ID] = employee
	}
	return lookup
}

func buildPayrollComponents(itemID string, profile EmployeeCompensationProfile, now time.Time, actorID string) []PayrollComponent {
	components := make([]PayrollComponent, 0, 6)
	appendComponent := func(componentType, code, name string, amount float64) {
		if amount <= 0 {
			return
		}
		components = append(components, PayrollComponent{
			Base:             Base{ID: uuid.New().String(), CreatedBy: actorID, CreatedAt: now, UpdatedAt: now},
			PayrollRunItemID: itemID,
			ComponentType:    componentType,
			Code:             code,
			Name:             name,
			Amount:           amount,
		})
	}
	appendComponent("earning", "BASE", "Base Salary", profile.BaseSalary)
	appendComponent("earning", "HOUSING", "Housing Allowance", profile.HousingAllowance)
	appendComponent("earning", "TRANSPORT", "Transport Allowance", profile.TransportAllowance)
	appendComponent("earning", "OTHER", "Other Allowance", profile.OtherAllowance)
	appendComponent("deduction", "STANDARD", "Standard Deduction", profile.StandardDeduction)
	appendComponent("deduction", "TAX", "Tax Deduction", profile.TaxDeduction)
	appendComponent("employer_cost", "EMPLOYER", "Employer Cost", profile.EmployerCost)
	return components
}

func newJournalLine(account ChartOfAccount, debit, credit float64, description string, now time.Time, actorID string) JournalLine {
	return JournalLine{
		Base:        Base{ID: uuid.New().String(), CreatedBy: actorID, CreatedAt: now, UpdatedAt: now},
		AccountID:   account.ID,
		AccountName: account.AccountName,
		Debit:       debit,
		Credit:      credit,
		Description: description,
	}
}

func generatePayrollRunNumber(period PayrollPeriod, now time.Time) string {
	return fmt.Sprintf("PAY-%d-%02d-%d", period.PeriodStart.Year(), int(period.PeriodStart.Month()), now.Unix()%10000)
}

func maxFinanceAmount(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func mergeNotes(existing, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if existing == "" {
		return next
	}
	if next == "" {
		return existing
	}
	return existing + "\n" + next
}

func parseOptionalISODateTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid date format")
}

func (a *App) getPayrollActorID() string {
	if current, err := a.GetCurrentEmployeeContext(); err == nil && current.EmployeeID != "" {
		return current.EmployeeID
	}
	if userID := a.getCurrentUserID(); strings.TrimSpace(userID) != "" {
		return userID
	}
	return "system"
}

func (a *App) emitPayrollEvent(name string, payload interface{}) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, payload)
}
