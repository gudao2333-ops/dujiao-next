package service

import (
	"regexp"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/shopspring/decimal"
)

var defaultSitePrefixRegex = `^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`

// SiteOpenSetting 子站开通设置
type SiteOpenSetting struct {
	Enabled           bool         `json:"site_open_enabled"`
	OpeningPrice      models.Money `json:"site_opening_price"`
	ProfitConfirmDays int          `json:"site_profit_confirm_days"`
	MinWithdrawAmount models.Money `json:"site_min_withdraw_amount"`
	WithdrawChannels  []string     `json:"site_withdraw_channels"`
	DomainSuffixes    []string     `json:"site_domain_suffixes"`
	ReservedPrefixes  []string     `json:"reserved_prefixes"`
	PrefixRegex       string       `json:"site_prefix_regex"`
}

func SiteOpenDefaultSetting() SiteOpenSetting {
	return SiteOpenSetting{
		Enabled:           false,
		OpeningPrice:      models.NewMoneyFromDecimal(decimal.Zero),
		ProfitConfirmDays: 0,
		MinWithdrawAmount: models.NewMoneyFromDecimal(decimal.Zero),
		WithdrawChannels:  []string{},
		DomainSuffixes:    []string{},
		ReservedPrefixes:  []string{"www", "admin", "api"},
		PrefixRegex:       defaultSitePrefixRegex,
	}
}

func normalizeSiteOpenSetting(setting SiteOpenSetting) SiteOpenSetting {
	if setting.OpeningPrice.Decimal.LessThan(decimal.Zero) {
		setting.OpeningPrice = models.NewMoneyFromDecimal(decimal.Zero)
	}
	if setting.ProfitConfirmDays < 0 {
		setting.ProfitConfirmDays = 0
	}
	if setting.MinWithdrawAmount.Decimal.LessThan(decimal.Zero) {
		setting.MinWithdrawAmount = models.NewMoneyFromDecimal(decimal.Zero)
	}
	if strings.TrimSpace(setting.PrefixRegex) == "" {
		setting.PrefixRegex = defaultSitePrefixRegex
	}
	if _, err := regexp.Compile(setting.PrefixRegex); err != nil {
		setting.PrefixRegex = defaultSitePrefixRegex
	}
	setting.DomainSuffixes = normalizeSiteStringList(setting.DomainSuffixes)
	setting.WithdrawChannels = normalizeSiteStringList(setting.WithdrawChannels)
	setting.ReservedPrefixes = normalizeSiteStringList(setting.ReservedPrefixes)
	return setting
}

func normalizeSiteStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func SiteOpenSettingToMap(setting SiteOpenSetting) map[string]interface{} {
	normalized := normalizeSiteOpenSetting(setting)
	return map[string]interface{}{
		"site_open_enabled":        normalized.Enabled,
		"site_opening_price":       normalized.OpeningPrice.String(),
		"site_profit_confirm_days": normalized.ProfitConfirmDays,
		"site_min_withdraw_amount": normalized.MinWithdrawAmount.String(),
		"site_withdraw_channels":   append([]string(nil), normalized.WithdrawChannels...),
		"site_domain_suffixes":     append([]string(nil), normalized.DomainSuffixes...),
		"reserved_prefixes":        append([]string(nil), normalized.ReservedPrefixes...),
		"site_prefix_regex":        normalized.PrefixRegex,
	}
}

func siteOpenSettingFromJSON(raw models.JSON, fallback SiteOpenSetting) SiteOpenSetting {
	result := fallback
	if raw == nil {
		return normalizeSiteOpenSetting(result)
	}
	if v, ok := raw["site_open_enabled"].(bool); ok {
		result.Enabled = v
	}
	if v, ok := raw["site_opening_price"]; ok {
		result.OpeningPrice = normalizeMoney(v)
	}
	if v, ok := raw["site_profit_confirm_days"].(float64); ok {
		result.ProfitConfirmDays = int(v)
	} else if v, ok := raw["site_profit_confirm_days"].(int); ok {
		result.ProfitConfirmDays = v
	}
	if v, ok := raw["site_min_withdraw_amount"]; ok {
		result.MinWithdrawAmount = normalizeMoney(v)
	}
	if v, ok := raw["site_withdraw_channels"].([]interface{}); ok {
		rows := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				rows = append(rows, text)
			}
		}
		result.WithdrawChannels = rows
	} else if v, ok := raw["site_withdraw_channels"].([]string); ok {
		result.WithdrawChannels = append([]string(nil), v...)
	}
	if v, ok := raw["site_domain_suffixes"].([]interface{}); ok {
		rows := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				rows = append(rows, text)
			}
		}
		result.DomainSuffixes = rows
	} else if v, ok := raw["site_domain_suffixes"].([]string); ok {
		result.DomainSuffixes = append([]string(nil), v...)
	}
	if v, ok := raw["reserved_prefixes"].([]interface{}); ok {
		rows := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				rows = append(rows, text)
			}
		}
		result.ReservedPrefixes = rows
	} else if v, ok := raw["reserved_prefixes"].([]string); ok {
		result.ReservedPrefixes = append([]string(nil), v...)
	}
	if v, ok := raw["site_prefix_regex"].(string); ok {
		result.PrefixRegex = strings.TrimSpace(v)
	}
	return normalizeSiteOpenSetting(result)
}

func normalizeMoney(v interface{}) models.Money {
	switch value := v.(type) {
	case string:
		if d, err := decimal.NewFromString(strings.TrimSpace(value)); err == nil {
			return models.NewMoneyFromDecimal(d)
		}
	case float64:
		return models.NewMoneyFromDecimal(decimal.NewFromFloat(value))
	case int:
		return models.NewMoneyFromDecimal(decimal.NewFromInt(int64(value)))
	}
	return models.NewMoneyFromDecimal(decimal.Zero)
}

func (s *SettingService) GetSiteOpenSetting() (SiteOpenSetting, error) {
	fallback := SiteOpenDefaultSetting()
	if s == nil {
		return fallback, nil
	}
	value, err := s.GetByKey(constants.SettingKeySiteOpenConfig)
	if err != nil {
		return fallback, err
	}
	if value == nil {
		return fallback, nil
	}
	return siteOpenSettingFromJSON(value, fallback), nil
}

func (s *SettingService) UpdateSiteOpenSetting(setting SiteOpenSetting) (SiteOpenSetting, error) {
	normalized := normalizeSiteOpenSetting(setting)
	if _, err := s.Update(constants.SettingKeySiteOpenConfig, SiteOpenSettingToMap(normalized)); err != nil {
		return SiteOpenDefaultSetting(), err
	}
	return normalized, nil
}
