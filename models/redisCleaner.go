package models

import (
	"github.com/mmdatafocus/books_backend/utils"
)

type RedisCleaner interface {
	RemoveInstanceRedis() error // remove one
	RemoveAllRedis() error      // remove list & map if exists
}

// remove both item & list + map
func RemoveRedisBoth[T RedisCleaner](obj T) error {
	if err := obj.RemoveInstanceRedis(); err != nil {
		return err
	}
	if err := obj.RemoveAllRedis(); err != nil {
		return err
	}
	return nil
}

/* admin resources */
func (obj State) RemoveInstanceRedis() error {
	return nil
}

func (obj State) RemoveAllRedis() error {
	return utils.ClearRedisAdmin[State]()
}

func (obj Township) RemoveInstanceRedis() error {
	return nil
}

func (obj Township) RemoveAllRedis() error {
	return utils.ClearRedisAdmin[Township]()
}

/* generated */
func (obj Account) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Account](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Account) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllAccount](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllAccount](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj Branch) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Branch](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Branch) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllBranch](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllBranch](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj Comment) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Comment](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Comment) RemoveAllRedis() error {
	return nil
}

func (obj Currency) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Currency](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Currency) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllCurrency](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllCurrency](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj CurrencyExchange) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[CurrencyExchange](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj CurrencyExchange) RemoveAllRedis() error {
	return nil
}

func (obj Customer) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Customer](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Customer) RemoveAllRedis() error {
	return nil
}

func (obj DeliveryMethod) RemoveInstanceRedis() error {
	return nil
}

func (obj DeliveryMethod) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllDeliveryMethod](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllDeliveryMethod](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj Journal) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Journal](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Journal) RemoveAllRedis() error {
	return nil
}

func (obj Module) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Module](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Module) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllModule](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj MoneyAccount) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[MoneyAccount](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj MoneyAccount) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllMoneyAccount](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllMoneyAccount](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj PaymentMode) RemoveInstanceRedis() error {
	return nil
}

func (obj PaymentMode) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllPaymentMode](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllPaymentMode](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj Product) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Product](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Product) RemoveAllRedis() error {
	return nil
}

func (obj ProductCategory) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[ProductCategory](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj ProductCategory) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllProductCategory](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllProductCategory](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj ProductGroup) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[ProductGroup](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj ProductGroup) RemoveAllRedis() error {
	return nil
}

func (obj ProductModifier) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[ProductModifier](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj ProductModifier) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllProductModifier](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllProductModifier](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj ProductUnit) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[ProductUnit](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj ProductUnit) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllProductUnit](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllProductUnit](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj ProductVariant) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[ProductVariant](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj ProductVariant) RemoveAllRedis() error {
	return nil
}

func (obj Reason) RemoveInstanceRedis() error {
	return nil
}

func (obj Reason) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllReason](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllReason](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj Role) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Role](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Role) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllRole](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllRole](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj SalesPerson) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[SalesPerson](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj SalesPerson) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllSalesPerson](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllSalesPerson](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj ShipmentPreference) RemoveInstanceRedis() error {
	return nil
}

func (obj ShipmentPreference) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllShipmentPreference](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllShipmentPreference](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj Supplier) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Supplier](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Supplier) RemoveAllRedis() error {
	return nil
}

func (obj Tax) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Tax](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Tax) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllTax](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllTax](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj TaxGroup) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[TaxGroup](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj TaxGroup) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllTaxGroup](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllTaxGroup](obj.BusinessId); err != nil {
		return err
	}
	return nil
}

func (obj TransactionNumberSeries) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[TransactionNumberSeries](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj TransactionNumberSeries) RemoveAllRedis() error {
	return nil
}

func (obj Warehouse) RemoveInstanceRedis() error {
	if err := utils.RemoveRedisItem[Warehouse](obj.ID); err != nil {
		return err
	}
	return nil
}

func (obj Warehouse) RemoveAllRedis() error {
	if err := utils.RemoveRedisList[AllWarehouse](obj.BusinessId); err != nil {
		return err
	}
	if err := utils.RemoveRedisMap[AllWarehouse](obj.BusinessId); err != nil {
		return err
	}
	return nil
}
