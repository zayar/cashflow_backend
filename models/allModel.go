package models

import (
	"context"
	"errors"
	"strconv"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// convert to tax info for showing result to user
func (t AllTax) Info() TaxInfo {
	return TaxInfo{
		ID:       string(TaxTypeIndividual) + strconv.Itoa(t.ID),
		Name:     t.Name,
		Rate:     t.Rate,
		Type:     TaxTypeIndividual,
		IsActive: t.IsActive,
	}
}

// convert to tax info for showing result to user
func (t AllTaxGroup) Info() TaxInfo {
	return TaxInfo{
		ID:       string(TaxTypeGroup) + strconv.Itoa(t.ID),
		Name:     t.Name,
		Rate:     t.Rate,
		Type:     TaxTypeIndividual,
		IsActive: t.IsActive,
	}
}

// get AllModelMap for loader, redis or db
func MapAllModel[ModelT any, AllT Identifier](ctx context.Context) (map[int]*AllT, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// retrieve from redis
	key := utils.GetTypeName[AllT]() + "Map:" + businessId

	var allMap map[int]*AllT

	// retrieve from redis
	if exists, err := config.GetRedisObject(key, &allMap); err != nil {
		return nil, err
	} else if !exists {
		// if the map has not been cached yet
		// fetch resources and constrcut the map, cache the result

		allMap = make(map[int]*AllT)
		var allSlice []*AllT
		db := config.GetDB()
		var m ModelT
		dbCtx := db.WithContext(ctx).Model(&m)
		dbCtx.Where("business_id = ?", businessId)
		if err := dbCtx.Find(&allSlice).Error; err != nil {
			return nil, err
		}

		// fill the map
		for _, allModel := range allSlice {
			allMap[(*allModel).GetId()] = allModel
		}

		// store redis
		var duration time.Duration
		// if utils.typeHasExpiration(typeName) {
		// 	duration = utils.GetCacheLifespan()
		// }
		if err := config.SetRedisObject(key, &allMap, duration); err != nil {
			return nil, err
		}
	}

	return allMap, nil
}

// get AllModelMap for loader, redis or db
func MapAllAdmin[ModelT any, AllT Identifier](ctx context.Context) (map[int]*AllT, error) {

	// retrieve from redis
	key := utils.GetTypeName[AllT]() + "Map"

	var allMap map[int]*AllT

	// retrieve from redis
	if exists, err := config.GetRedisObject(key, &allMap); err != nil {
		return nil, err
	} else if !exists {
		// if the map has not been cached yet
		// fetch resources and constrcut the map, cache the result

		allMap = make(map[int]*AllT)
		var allSlice []*AllT
		db := config.GetDB()
		var m ModelT
		dbCtx := db.WithContext(ctx).Model(&m)
		if err := dbCtx.Find(&allSlice).Error; err != nil {
			return nil, err
		}

		// fill the map
		for _, allModel := range allSlice {
			allMap[(*allModel).GetId()] = allModel
		}

		// store redis
		var duration time.Duration
		if err := config.SetRedisObject(key, &allMap, duration); err != nil {
			return nil, err
		}
	}

	return allMap, nil
}

// embedding struct will receive ID field, satisfy Identifier interface
type HasId struct {
	ID int `json:"id"`
}

func (h HasId) GetId() int {
	return h.ID
}

//	func (h HasId) GetId() int {
//		return h.ID
//	}
type HasUid struct {
	ID uuid.UUID `json:"id"`
}

func (h HasUid) GetId() uuid.UUID {
	return h.ID
}

type AllAccount struct {
	HasId
	Name              string            `json:"name"`
	Code              string            `json:"code"`
	DetailType        AccountDetailType `json:"detail_type"`
	MainType          AccountMainType   `json:"main_type"`
	IsActive          bool              `json:"is_active"`
	SystemDefaultCode string            `json:"system_default_code"`
	CurrencyId        int               `json:"currency_id"`
}

type AllBranch struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type AllBusiness struct {
	HasUid
	LogoURL  string `json:"logoUrl"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	IsActive bool   `json:"is_active"`
	Address  string `json:"address"`
	Country  string `json:"country"`
	City     string `json:"city"`
}

type AllCurrency struct {
	HasId
	DecimalPlaces DecimalPlaces `json:"decimalPlaces"`
	Name          string        `json:"name"`
	Symbol        string        `json:"symbol"`
	IsActive      bool          `json:"is_active"`
}

type AllDeliveryMethod struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type AllMoneyAccount struct {
	HasId
	AccountName string `json:"accountName"`
	IsActive    bool   `json:"is_active"`
}

type AllPaymentMode struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type AllProductCategory struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"isActive"`
}

type AllProductModifier struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"isActive"`
}

type AllProductUnit struct {
	HasId
	Name         string    `json:"name"`
	Abbreviation string    `json:"abbreviation"`
	Precision    Precision `json:"precision"`
	IsActive     bool      `json:"isActive"`
}

type AllReason struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type AllRole struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type AllSalesPerson struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type AllShipmentPreference struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type AllState struct {
	HasId
	Code        string `json:"code"`
	StateNameEn string `json:"stateNameEn"`
	StateNameMm string `json:"stateNameMm"`
	IsActive    bool   `json:"is_active"`
}

type AllTax struct {
	HasId
	IsCompoundTax bool            `json:"isCompoundTax"`
	Name          string          `json:"name"`
	Rate          decimal.Decimal `json:"rate"`
	IsActive      bool            `json:"is_active"`
}

type AllTaxGroup struct {
	HasId
	Name     string          `json:"name"`
	Rate     decimal.Decimal `json:"rate"`
	IsActive bool            `json:"is_active"`
}

type AllTownship struct {
	HasId
	Code           string `json:"code"`
	StateCode      string `json:"stateCode"`
	TownshipNameEn string `json:"townshipNameEn"`
	TownshipNameMm string `json:"townshipNameMm"`
	IsActive       bool   `json:"is_active"`
}

type AllUser struct {
	HasId
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type AllWarehouse struct {
	HasId
	Name     string `json:"name"`
	BranchId int    `json:"branch_id"`
	IsActive bool   `json:"is_active"`
}

func ListAllAccount(ctx context.Context) ([]*AllAccount, error) {
	return ListAllResource[Account, AllAccount](ctx)
}

func MapAllAccount(ctx context.Context) (map[int]*AllAccount, error) {
	return MapAllModel[Account, AllAccount](ctx)
}

func ListAllBranch(ctx context.Context) ([]*AllBranch, error) {
	return ListAllResource[Branch, AllBranch](ctx)
}

func MapAllBranch(ctx context.Context) (map[int]*AllBranch, error) {
	return MapAllModel[Branch, AllBranch](ctx)
}

func ListAllCurrency(ctx context.Context) ([]*AllCurrency, error) {
	return ListAllResource[Currency, AllCurrency](ctx)
}

func MapAllCurrency(ctx context.Context) (map[int]*AllCurrency, error) {
	return MapAllModel[Currency, AllCurrency](ctx)
}

func ListAllDeliveryMethod(ctx context.Context) ([]*AllDeliveryMethod, error) {
	return ListAllResource[DeliveryMethod, AllDeliveryMethod](ctx)
}

func MapAllDeliveryMethod(ctx context.Context) (map[int]*AllDeliveryMethod, error) {
	return MapAllModel[DeliveryMethod, AllDeliveryMethod](ctx)
}

func ListAllMoneyAccount(ctx context.Context) ([]*AllMoneyAccount, error) {
	return ListAllResource[MoneyAccount, AllMoneyAccount](ctx)
}

func MapAllMoneyAccount(ctx context.Context) (map[int]*AllMoneyAccount, error) {
	return MapAllModel[MoneyAccount, AllMoneyAccount](ctx)
}

func ListAllPaymentMode(ctx context.Context) ([]*AllPaymentMode, error) {
	return ListAllResource[PaymentMode, AllPaymentMode](ctx)
}

func MapAllPaymentMode(ctx context.Context) (map[int]*AllPaymentMode, error) {
	return MapAllModel[PaymentMode, AllPaymentMode](ctx)
}

func ListAllProductCategory(ctx context.Context) ([]*AllProductCategory, error) {
	return ListAllResource[ProductCategory, AllProductCategory](ctx)
}

func MapAllProductCategory(ctx context.Context) (map[int]*AllProductCategory, error) {
	return MapAllModel[ProductCategory, AllProductCategory](ctx)
}

func ListAllProductModifier(ctx context.Context) ([]*AllProductModifier, error) {
	return ListAllResource[ProductModifier, AllProductModifier](ctx)
}

func MapAllProductModifier(ctx context.Context) (map[int]*AllProductModifier, error) {
	return MapAllModel[ProductModifier, AllProductModifier](ctx)
}

func ListAllProductUnit(ctx context.Context) ([]*AllProductUnit, error) {
	return ListAllResource[ProductUnit, AllProductUnit](ctx)
}

func MapAllProductUnit(ctx context.Context) (map[int]*AllProductUnit, error) {
	return MapAllModel[ProductUnit, AllProductUnit](ctx)
}

func ListAllReason(ctx context.Context) ([]*AllReason, error) {
	return ListAllResource[Reason, AllReason](ctx)
}

func MapAllReason(ctx context.Context) (map[int]*AllReason, error) {
	return MapAllModel[Reason, AllReason](ctx)
}

func ListAllRole(ctx context.Context) ([]*AllRole, error) {
	return ListAllResource[Role, AllRole](ctx)
}

func MapAllRole(ctx context.Context) (map[int]*AllRole, error) {
	return MapAllModel[Role, AllRole](ctx)
}

func ListAllSalesPerson(ctx context.Context) ([]*AllSalesPerson, error) {
	return ListAllResource[SalesPerson, AllSalesPerson](ctx)
}

func MapAllSalesPerson(ctx context.Context) (map[int]*AllSalesPerson, error) {
	return MapAllModel[SalesPerson, AllSalesPerson](ctx)
}

func ListAllShipmentPreference(ctx context.Context) ([]*AllShipmentPreference, error) {
	return ListAllResource[ShipmentPreference, AllShipmentPreference](ctx)
}

func MapAllShipmentPreference(ctx context.Context) (map[int]*AllShipmentPreference, error) {
	return MapAllModel[ShipmentPreference, AllShipmentPreference](ctx)
}

func ListAllState(ctx context.Context) ([]*AllState, error) {
	return ListAllAdmin[State, AllState](ctx)
}

func MapAllState(ctx context.Context) (map[int]*AllState, error) {
	return MapAllAdmin[State, AllState](ctx)
}
func ListAllBusiness(ctx context.Context) ([]*AllBusiness, error) {
	return ListAllAdmin[Business, AllBusiness](ctx)
}
func ListAllTax(ctx context.Context) ([]*AllTax, error) {
	return ListAllResource[Tax, AllTax](ctx)
}

func MapAllTax(ctx context.Context) (map[int]*AllTax, error) {
	return MapAllModel[Tax, AllTax](ctx)
}

func ListAllTaxGroup(ctx context.Context) ([]*AllTaxGroup, error) {
	return ListAllResource[TaxGroup, AllTaxGroup](ctx)
}

func MapAllTaxGroup(ctx context.Context) (map[int]*AllTaxGroup, error) {
	return MapAllModel[TaxGroup, AllTaxGroup](ctx)
}

func ListAllTownship(ctx context.Context) ([]*AllTownship, error) {
	return ListAllAdmin[Township, AllTownship](ctx)
}

func MapAllTownship(ctx context.Context) (map[int]*AllTownship, error) {
	return MapAllAdmin[Township, AllTownship](ctx)
}

func ListAllUser(ctx context.Context) ([]*AllUser, error) {
	return ListAllResource[User, AllUser](ctx)
}

func MapAllUser(ctx context.Context) (map[int]*AllUser, error) {
	return MapAllModel[User, AllUser](ctx)
}

func ListAllWarehouse(ctx context.Context) ([]*AllWarehouse, error) {
	return ListAllResource[Warehouse, AllWarehouse](ctx)
}

func MapAllWarehouse(ctx context.Context) (map[int]*AllWarehouse, error) {
	return MapAllModel[Warehouse, AllWarehouse](ctx)
}
