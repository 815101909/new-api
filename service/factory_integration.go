package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	factoryAccountsPath = "/api/internal/new-api/accounts"
	factoryBindPath     = "/api/internal/new-api/token-bind"
)

type FactoryAccount struct {
	Id          int                          `json:"id"`
	Username    string                       `json:"username"`
	DisplayName string                       `json:"display_name"`
	Email       string                       `json:"email"`
	Phone       string                       `json:"phone"`
	Role        string                       `json:"role"`
	Status      int                          `json:"status"`
	CreatedAt   int64                        `json:"created_at"`
	Bound       bool                         `json:"bound"`
	Binding     *model.FactoryAccountBinding `json:"binding,omitempty"`
}

type FactoryAccountListRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword"`
}

type FactoryAccountListResponse struct {
	Rows     []FactoryAccount `json:"rows"`
	Count    int              `json:"count"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type FactoryImportRequest struct {
	FactoryUserIds []int `json:"factory_user_ids"`
	ProviderIds    []int `json:"provider_ids"`
}

type FactoryImportResult struct {
	FactoryUserId   int    `json:"factory_user_id"`
	FactoryUsername string `json:"factory_username"`
	NewApiUserId    int    `json:"new_api_user_id"`
	NewApiUsername  string `json:"new_api_username"`
	TokenId         int    `json:"token_id"`
	Success         bool   `json:"success"`
	Message         string `json:"message"`
}

type FactoryImportResponse struct {
	Rows []FactoryImportResult `json:"rows"`
}

type factoryAccountsRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword"`
	Ids      []int  `json:"ids,omitempty"`
}

type factoryBindRequest struct {
	FactoryUserId  int    `json:"factory_user_id"`
	ProviderIds    []int  `json:"provider_ids,omitempty"`
	ApiKey         string `json:"api_key"`
	NewApiUserId   int    `json:"new_api_user_id"`
	NewApiUsername string `json:"new_api_username"`
	NewApiTokenId  int    `json:"new_api_token_id"`
}

type factoryEnvelope[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func GetFactoryIntegrationStatus() map[string]any {
	baseURL := strings.TrimSpace(os.Getenv("AIGC_FACTORY_BASE_URL"))
	secret := strings.TrimSpace(os.Getenv("AIGC_FACTORY_SYNC_SECRET"))
	providerIds := parseFactoryProviderIds(os.Getenv("AIGC_FACTORY_PROVIDER_IDS"))
	return map[string]any{
		"configured":   baseURL != "" && secret != "",
		"base_url":     baseURL,
		"provider_ids": providerIds,
	}
}

func ListFactoryAccounts(ctx context.Context, req FactoryAccountListRequest) (*FactoryAccountListResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}
	body := factoryAccountsRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
		Keyword: strings.TrimSpace(req.Keyword),
	}
	accounts, err := fetchFactoryAccounts(ctx, body)
	if err != nil {
		return nil, err
	}
	if err := attachFactoryBindings(accounts.Rows); err != nil {
		return nil, err
	}
	return accounts, nil
}

func ListFactoryBindings(page int, pageSize int) (*common.PageInfo, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = common.ItemsPerPage
	}
	pageInfo := &common.PageInfo{Page: page, PageSize: pageSize}
	bindings, total, err := model.ListFactoryAccountBindings(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		return nil, err
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(bindings)
	return pageInfo, nil
}

func ImportFactoryAccounts(ctx context.Context, req FactoryImportRequest) (*FactoryImportResponse, error) {
	factoryUserIds := uniquePositiveInts(req.FactoryUserIds)
	if len(factoryUserIds) == 0 {
		return nil, errors.New("请选择要导入的创作工厂账号")
	}
	providerIds := uniquePositiveInts(req.ProviderIds)
	if len(providerIds) == 0 {
		providerIds = parseFactoryProviderIds(os.Getenv("AIGC_FACTORY_PROVIDER_IDS"))
	}
	accountsResp, err := fetchFactoryAccounts(ctx, factoryAccountsRequest{
		Page:     1,
		PageSize: len(factoryUserIds),
		Ids:      factoryUserIds,
	})
	if err != nil {
		return nil, err
	}
	accountMap := make(map[int]FactoryAccount, len(accountsResp.Rows))
	for _, account := range accountsResp.Rows {
		accountMap[account.Id] = account
	}
	rows := make([]FactoryImportResult, 0, len(factoryUserIds))
	for _, factoryUserId := range factoryUserIds {
		account, ok := accountMap[factoryUserId]
		if !ok {
			rows = append(rows, FactoryImportResult{
				FactoryUserId: factoryUserId,
				Success:       false,
				Message:       "创作工厂账号不存在",
			})
			continue
		}
		result := importFactoryAccount(ctx, account, providerIds)
		rows = append(rows, result)
	}
	return &FactoryImportResponse{Rows: rows}, nil
}

func importFactoryAccount(ctx context.Context, account FactoryAccount, providerIds []int) FactoryImportResult {
	result := FactoryImportResult{
		FactoryUserId:   account.Id,
		FactoryUsername: account.Username,
	}
	user, token, err := ensureFactoryUserAndToken(account)
	if err != nil {
		result.Success = false
		result.Message = err.Error()
		return result
	}
	result.NewApiUserId = user.Id
	result.NewApiUsername = user.Username
	result.TokenId = token.Id
	apiKey := withSkPrefix(token.GetFullKey())
	err = bindFactoryToken(ctx, account.Id, providerIds, apiKey, user, token)
	if err != nil {
		result.Success = false
		result.Message = err.Error()
		return result
	}
	err = model.SaveFactoryAccountBinding(&model.FactoryAccountBinding{
		FactoryUserId:      account.Id,
		FactoryUsername:    account.Username,
		FactoryDisplayName: account.DisplayName,
		FactoryRole:        account.Role,
		FactoryStatus:      account.Status,
		NewApiUserId:       user.Id,
		NewApiUsername:     user.Username,
		TokenId:            token.Id,
		Status:             model.FactoryAccountBindingStatusEnabled,
		LastSyncedAt:       common.GetTimestamp(),
	})
	if err != nil {
		result.Success = false
		result.Message = err.Error()
		return result
	}
	result.Success = true
	result.Message = "导入成功"
	return result
}

func fetchFactoryAccounts(ctx context.Context, req factoryAccountsRequest) (*FactoryAccountListResponse, error) {
	var data FactoryAccountListResponse
	if err := callFactoryAPI(ctx, factoryAccountsPath, req, &data); err != nil {
		return nil, err
	}
	if data.Page <= 0 {
		data.Page = req.Page
	}
	if data.PageSize <= 0 {
		data.PageSize = req.PageSize
	}
	return &data, nil
}

func bindFactoryToken(ctx context.Context, factoryUserId int, providerIds []int, apiKey string, user *model.User, token *model.Token) error {
	req := factoryBindRequest{
		FactoryUserId:  factoryUserId,
		ProviderIds:    providerIds,
		ApiKey:         apiKey,
		NewApiUserId:   user.Id,
		NewApiUsername: user.Username,
		NewApiTokenId:  token.Id,
	}
	var data map[string]any
	return callFactoryAPI(ctx, factoryBindPath, req, &data)
}

func callFactoryAPI[T any](ctx context.Context, path string, payload any, data *T) error {
	baseURL, secret, err := factoryConfig()
	if err != nil {
		return err
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	timeout := common.GetEnvOrDefault("AIGC_FACTORY_SYNC_TIMEOUT_SECONDS", 15)
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	url := baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-New-API-Sync-Secret", secret)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求创作工厂失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("创作工厂返回异常状态 %d: %s", resp.StatusCode, strings.TrimSpace(string(buf)))
	}
	var envelope factoryEnvelope[T]
	if err := common.DecodeJson(resp.Body, &envelope); err != nil {
		return fmt.Errorf("解析创作工厂响应失败: %w", err)
	}
	if !envelope.Success {
		if envelope.Message == "" {
			envelope.Message = "创作工厂接口返回失败"
		}
		return errors.New(envelope.Message)
	}
	if data != nil {
		*data = envelope.Data
	}
	return nil
}

func ensureFactoryUserAndToken(account FactoryAccount) (*model.User, *model.Token, error) {
	if binding, err := model.GetFactoryAccountBindingByFactoryUserId(account.Id); err == nil {
		user, token, reuseErr := reuseBoundUserAndToken(binding)
		if reuseErr == nil {
			return user, token, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}
	user, err := ensureFactoryUser(account)
	if err != nil {
		return nil, nil, err
	}
	token, err := ensureFactoryToken(user)
	if err != nil {
		return nil, nil, err
	}
	return user, token, nil
}

func reuseBoundUserAndToken(binding *model.FactoryAccountBinding) (*model.User, *model.Token, error) {
	user, err := model.GetUserById(binding.NewApiUserId, true)
	if err != nil {
		return nil, nil, err
	}
	token, err := model.GetTokenById(binding.TokenId)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureFactoryTokenUsable(token, user.Group); err != nil {
		return nil, nil, err
	}
	return user, token, nil
}

func ensureFactoryUser(account FactoryAccount) (*model.User, error) {
	username := factoryUsername(account.Id)
	var user model.User
	err := model.DB.Where("username = ?", username).First(&user).Error
	if err == nil {
		if user.Status != common.UserStatusEnabled || user.Group == "" {
			updates := map[string]any{
				"status": common.UserStatusEnabled,
			}
			if user.Group == "" {
				updates["group"] = "default"
				user.Group = "default"
			}
			if err := model.DB.Model(&user).Updates(updates).Error; err != nil {
				return nil, err
			}
			user.Status = common.UserStatusEnabled
		}
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	password, err := common.GenerateRandomCharsKey(16)
	if err != nil {
		return nil, err
	}
	hashedPassword, err := common.Password2Hash(password)
	if err != nil {
		return nil, err
	}
	displayName := truncateRunes(firstNonEmpty(account.DisplayName, account.Username, username), 20)
	user = model.User{
		Username:    username,
		Password:    hashedPassword,
		DisplayName: displayName,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		Quota:       0,
		UsedQuota:   0,
		AffCode:     common.GetRandomString(4),
		Remark:      fmt.Sprintf("Creative Factory user #%d", account.Id),
	}
	if err := model.DB.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func ensureFactoryToken(user *model.User) (*model.Token, error) {
	var token model.Token
	err := model.DB.Where("user_id = ? AND name = ?", user.Id, "Creative Factory").First(&token).Error
	if err == nil {
		if err := ensureFactoryTokenUsable(&token, user.Group); err != nil {
			return nil, err
		}
		return &token, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	token = model.Token{
		UserId:         user.Id,
		Name:           "Creative Factory",
		Key:            key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    now,
		AccessedTime:   now,
		ExpiredTime:    -1,
		RemainQuota:    0,
		UnlimitedQuota: true,
		Group:          user.Group,
	}
	if err := token.Insert(); err != nil {
		return nil, err
	}
	return &token, nil
}

func ensureFactoryTokenUsable(token *model.Token, group string) error {
	changed := false
	if token.Status != common.TokenStatusEnabled {
		token.Status = common.TokenStatusEnabled
		changed = true
	}
	if token.ExpiredTime != -1 {
		token.ExpiredTime = -1
		changed = true
	}
	if !token.UnlimitedQuota {
		token.UnlimitedQuota = true
		changed = true
	}
	if token.Group != group {
		token.Group = group
		changed = true
	}
	if !changed {
		return nil
	}
	return token.Update()
}

func attachFactoryBindings(accounts []FactoryAccount) error {
	ids := make([]int, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.Id)
	}
	bindingMap, err := model.GetFactoryAccountBindingMap(ids)
	if err != nil {
		return err
	}
	for index := range accounts {
		if binding, ok := bindingMap[accounts[index].Id]; ok {
			accounts[index].Bound = true
			accounts[index].Binding = binding
		}
	}
	return nil
}

func factoryConfig() (string, string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("AIGC_FACTORY_BASE_URL")), "/")
	secret := strings.TrimSpace(os.Getenv("AIGC_FACTORY_SYNC_SECRET"))
	if baseURL == "" || secret == "" {
		return "", "", errors.New("请配置 AIGC_FACTORY_BASE_URL 和 AIGC_FACTORY_SYNC_SECRET")
	}
	return baseURL, secret, nil
}

func factoryUsername(factoryUserId int) string {
	return fmt.Sprintf("cf_%d", factoryUserId)
}

func withSkPrefix(key string) string {
	if strings.HasPrefix(key, "sk-") {
		return key
	}
	return "sk-" + key
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes])
}

func uniquePositiveInts(values []int) []int {
	result := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func parseFactoryProviderIds(value string) []int {
	parts := strings.Split(value, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			continue
		}
		result = append(result, id)
	}
	return uniquePositiveInts(result)
}
