package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"

	"github.com/99designs/gqlgen/graphql"
	"github.com/bsm/redislock"
	"github.com/go-playground/validator/v10"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/shopspring/decimal"
	"github.com/ttacon/libphonenumber"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var CountryCode = "MM"

func IsValidEmail(email string) bool {
	// Basic email validation regex pattern
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	regex := regexp.MustCompile(pattern)
	return regex.MatchString(email)
}

func IsRecordValidByID(id uint, model interface{}, db *gorm.DB) bool {

	modelType := reflect.TypeOf(model).Elem() // Get the type of the element (struct)
	record := reflect.New(modelType).Interface()
	// Construct a query using the model's primary key
	query := db.Where("id = ?", id)

	// Perform the query
	if err := query.First(record).Error; err != nil {
		return false // Record with the given ID does not exist
	}

	return true
}

func ValidatePhoneNumber(phoneNumber, countryCode string) error {
	p, err := libphonenumber.Parse(phoneNumber, countryCode)
	if err != nil {
		return err // Phone number is invalid
	}

	if !libphonenumber.IsValidNumber(p) {
		return fmt.Errorf("phone number is not valid")
	}

	return nil // Phone number is valid for the specified country code
}

func GenerateUniqueFilename() string {

	timestamp := time.Now().UnixNano()

	random := rand.Intn(1000)

	uniqueFilename := fmt.Sprintf("%d_%d", timestamp, random)

	return uniqueFilename
}

func ProcessValidationErrors(err error) map[string]string {

	validationErrors := err.(validator.ValidationErrors)

	errorResponse := make(map[string]string)

	for _, ve := range validationErrors {
		errorResponse[ve.Field()] = ve.Tag()
	}

	return errorResponse
}

func NewTrue() *bool {
	b := true
	return &b
}

func NewFalse() *bool {
	b := false
	return &b
}

func GetQueryFields(ctx context.Context, model interface{}) (fieldNames []string, err error) {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return
	}
	m := make(map[string]string)
	for _, field := range s.Fields {
		dbName := field.DBName
		modelName := strings.ToLower(field.Name)
		m[modelName] = dbName
	}

	fields := graphql.CollectFieldsCtx(ctx, nil)
	for _, column := range fields {
		if !strings.HasPrefix(column.Name, "__") {
			colName := strings.ToLower(column.Name)
			if len(column.Selections) == 0 {
				fieldNames = append(fieldNames, m[colName])
			} else {
				dbName := m[colName+"id"]
				if len(dbName) > 0 {
					colName += "id"
				} else {
					colName += "code"
				}
				fieldNames = append(fieldNames, m[colName])
			}
		}
	}
	return
}

func GetPaginatedQueryFields(ctx context.Context, model interface{}) (fieldNames []string, err error) {
	s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return
	}
	m := make(map[string]string)
	for _, field := range s.Fields {
		dbName := field.DBName
		modelName := strings.ToLower(field.Name)
		m[modelName] = dbName
	}

	fields := graphql.CollectFieldsCtx(ctx, nil)
	for _, column := range fields {
		if column.Name == "edges" {
			edgesFields := graphql.CollectFields(graphql.GetOperationContext(ctx), column.Selections, nil)
			nodeFields := graphql.CollectFields(graphql.GetOperationContext(ctx), edgesFields[0].Selections, nil)
			for _, nodeColumn := range nodeFields {
				if !strings.HasPrefix(nodeColumn.Name, "__") {
					colName := strings.ToLower(nodeColumn.Name)
					if len(nodeColumn.Selections) == 0 {
						fieldNames = append(fieldNames, m[colName])
					} else {
						dbName := m[colName+"id"]
						if len(dbName) > 0 {
							colName += "id"
						} else {
							colName += "code"
						}
						fieldNames = append(fieldNames, m[colName])
					}
				}
			}
			break
		}
	}
	return
}

func ConvertToLocalTime(utcTime time.Time, timezone string) time.Time {
	//init the loc
	loc, _ := time.LoadLocation(timezone)
	//set timezone,
	return utcTime.In(loc)
}

func SplitQueryPath(path string) (module string, action string, err error) {
	var capitalIndex int
	for i, r := range path {
		if unicode.IsUpper(r) {
			capitalIndex = i
			break
		}
	}
	if capitalIndex == 0 {
		err = errors.New("invalid query")
		return
	}
	action = path[:capitalIndex]
	module = path[capitalIndex:]
	return
}

// returns slice removing duplicate elements
func UniqueSlice[T comparable](slice []T) []T {
	inResult := make(map[T]bool)
	var result []T
	for _, elm := range slice {
		if _, ok := inResult[elm]; !ok {
			// if not exists in map, append it, otherwise do nothing
			inResult[elm] = true
			result = append(result, elm)
		}
	}
	return result
}

// GetFromDateFromFiscalYear calculates the fromDate based on the toDate and fiscal year
func GetFromDateFromFiscalYear(toDate time.Time, fiscalYear string) (time.Time, error) {
	// Get the year from toDate
	toDateMonth := toDate.Month()
	toDateYear := toDate.Year()

	// Construct a mapping of month names to their corresponding numerical representations
	monthMap := map[string]time.Month{
		"Jan": time.January,
		"Feb": time.February,
		"Mar": time.March,
		"Apr": time.April,
		"May": time.May,
		"Jun": time.June,
		"Jul": time.July,
		"Aug": time.August,
		"Sep": time.September,
		"Oct": time.October,
		"Nov": time.November,
		"Dec": time.December,
	}

	// Check if the fiscalYear is a valid key in the monthMap
	month, ok := monthMap[fiscalYear]
	if !ok {
		return time.Time{}, errors.New("invalid month")
	}

	// Construct the fromDate with the same year as toDate and the month extracted from fiscalYear, and day 1
	fromDate := time.Date(toDateYear, month, 1, 0, 0, 0, 0, time.UTC)
	if month > toDateMonth {
		fromDate = time.Date(toDateYear-1, month, 1, 0, 0, 0, 0, time.UTC)
	}

	return fromDate, nil
}

func GetFiscalYearRange(fiscalYearStartMonth time.Month, year int) (time.Time, time.Time) {
	start := time.Date(year, fiscalYearStartMonth, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, -1).Add(time.Hour*23 + time.Minute*59 + time.Second*59)
	return start, end
}

func GetPreviousFiscalYearRange(fiscalYearStartMonth time.Month, year int) (time.Time, time.Time) {
	return GetFiscalYearRange(fiscalYearStartMonth, year-1)
}

func GetLastMonthsRange(months int) (time.Time, time.Time) {
	now := time.Now()
	start := now.AddDate(0, -months, 0)
	return start, now
}

// GetThisMonthRange returns the start and end dates of the current month.
func GetThisMonthRange() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, -1).Add(time.Hour*23 + time.Minute*59 + time.Second*59)
	return start, end
}

// GetPreviousMonthRange returns the start and end dates of the previous month.
func GetPreviousMonthRange() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, -1).Add(time.Hour*23 + time.Minute*59 + time.Second*59)
	return start, end
}

// GetQuarterRange returns the start and end dates for the quarter containing the specified month.
func GetQuarterRange(year int, month time.Month) (time.Time, time.Time) {
	startMonth := ((int(month)-1)/3)*3 + 1
	start := time.Date(year, time.Month(startMonth), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 3, -1).Add(time.Hour*23 + time.Minute*59 + time.Second*59)
	return start, end
}

// GetThisQuarterRange returns the start and end dates of the current quarter.
func GetThisQuarterRange() (time.Time, time.Time) {
	now := time.Now()
	return GetQuarterRange(now.Year(), now.Month())
}

// GetPreviousQuarterRange returns the start and end dates of the previous quarter.
func GetPreviousQuarterRange() (time.Time, time.Time) {
	now := time.Now()
	previousQuarterMonth := time.Month(((int(now.Month())-1)/3)*3 + 1 - 3)
	if previousQuarterMonth <= 0 {
		return GetQuarterRange(now.Year()-1, previousQuarterMonth+12)
	}
	return GetQuarterRange(now.Year(), previousQuarterMonth)
}

// to get the current fiscal year start month
func GetFiscalYearStartMonth(fiscalYear string) (time.Month, error) {
	switch fiscalYear {
	case "Jan":
		return time.January, nil
	case "Feb":
		return time.February, nil
	case "Mar":
		return time.March, nil
	case "Apr":
		return time.April, nil
	case "May":
		return time.May, nil
	case "Jun":
		return time.June, nil
	case "Jul":
		return time.July, nil
	case "Aug":
		return time.August, nil
	case "Sep":
		return time.September, nil
	case "Oct":
		return time.October, nil
	case "Nov":
		return time.November, nil
	case "Dec":
		return time.December, nil
	default:
		return 0, errors.New("invalid fiscal year month")
	}
}

// get the start and end dates based on the filter type and business fiscal year
func GetStartAndEndDateWithBusinessFiscalYear(fiscalYearStartMonth time.Month, filterType string) (time.Time, time.Time, error) {
	var startDate, endDate time.Time
	currentYear := time.Now().Year()

	switch filterType {
	case "last6months":
		startDate, endDate = GetLastMonthsRange(6)
	case "last12months":
		startDate, endDate = GetLastMonthsRange(12)
	case "thisFiscalYear":
		startDate, endDate = GetFiscalYearRange(fiscalYearStartMonth, currentYear)
		if time.Now().Before(startDate) {
			startDate, endDate = GetFiscalYearRange(fiscalYearStartMonth, currentYear-1)
		}
	case "previousFiscalYear":
		startDate, endDate = GetPreviousFiscalYearRange(fiscalYearStartMonth, currentYear)
	case "thisMonth":
		startDate, endDate = GetThisMonthRange()
	case "previousMonth":
		startDate, endDate = GetPreviousMonthRange()
	case "thisQuarter":
		startDate, endDate = GetThisQuarterRange()
	case "previousQuarter":
		startDate, endDate = GetPreviousQuarterRange()
	default:
		return time.Time{}, time.Time{}, errors.New("invalid filter type")
	}

	return startDate, endDate, nil
}

type DetailFieldValues struct {
	ProductIDs   []int
	ProductTypes []string
	BatchNumbers []string
}

type DetailField struct {
	ProductID   int
	ProductType string
	BatchNumber string
}

func FetchDetailFieldValues(tx *gorm.DB, model interface{}, idField string, idValue interface{}) (*DetailFieldValues, error) {

	var results []DetailField

	if err := tx.Model(model).
		Select("product_id, product_type, batch_number").
		Where(fmt.Sprintf("%s = ?", idField), idValue).
		Scan(&results).Error; err != nil {
		return nil, err
	}

	productIDs := make([]int, len(results))
	productTypes := make([]string, len(results))
	batchNumbers := make([]string, len(results))

	for i, result := range results {
		productIDs[i] = result.ProductID
		productTypes[i] = result.ProductType
		batchNumbers[i] = result.BatchNumber
	}

	return &DetailFieldValues{
		ProductIDs:   productIDs,
		ProductTypes: productTypes,
		BatchNumbers: batchNumbers,
	}, nil
}

// func FetchFieldValues(tx *gorm.DB, model interface{}, idField string, idValue interface{}) (*FieldValues, error) {
//     var productIDs []int
//     var productTypes []string
//     var batchNumbers []string

// 	// if err := tx.Model(model).
// 		// 	Select("product_id, product_type, batch_number").
// 		// 	Where(fmt.Sprintf("%s = ?", idField), idValue).
// 		// 	Scan(&results).Error; err != nil {
// 		// 	return nil, err
// 		// }

//     if err := tx.Model(model).Where(fmt.Sprintf("%s = ?", idField), idValue).Pluck("product_id", &productIDs).Error; err != nil {
//         return nil, err
//     }
//     if err := tx.Model(model).Where(fmt.Sprintf("%s = ?", idField), idValue).Pluck("product_type", &productTypes).Error; err != nil {
//         return nil, err
//     }
//     if err := tx.Model(model).Where(fmt.Sprintf("%s = ?", idField), idValue).Pluck("batch_number", &batchNumbers).Error; err != nil {
//         return nil, err
//     }

//     return &FieldValues{
//         ProductIDs:   productIDs,
//         ProductTypes: productTypes,
//         BatchNumbers: batchNumbers,
//     }, nil
// }

// execute given template string and return generated string
func ExecTemplate(tString string, data map[string]interface{}) (string, error) {
	t, err := template.New("sql").Parse(tString)
	if err != nil {
		return "", errors.New("error parsing sql template: " + err.Error())
	}
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return "", errors.New("failed to execute sql template: " + err.Error())
	}
	return b.String(), nil
}

// safely dereference pointer of type T, nil pointer return zero value or optional default
func DereferencePtr[T any](ptr *T, defaults ...T) T {
	var defaultValue T
	if len(defaults) > 0 {
		defaultValue = defaults[0]
	}
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// return nil if boolean expression is true, else the given default
func NilOrElse[T any](b bool, elseValue T) *T {
	if b {
		return nil
	}
	return &elseValue
}

func NilIfEmpty[T comparable](ptr T) *T {
	var defaultZero T
	if ptr == defaultZero {
		return nil
	}
	return &ptr
}

// turn salesInvoice to SalesInvoice
func UppercaseFirst(s string) string {
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// turn ToggleActive to toggleActive
func LowercaseFirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// mergeSlices merges two integer slices and removes duplicates
func MergeIntSlices(slice1, slice2 []int) []int {
	elementMap := make(map[int]bool)
	mergedSlice := []int{}

	// Add elements from the first slice to the map
	for _, elem := range slice1 {
		if !elementMap[elem] {
			elementMap[elem] = true
			mergedSlice = append(mergedSlice, elem)
		}
	}

	// Add elements from the second slice to the map
	for _, elem := range slice2 {
		if !elementMap[elem] {
			elementMap[elem] = true
			mergedSlice = append(mergedSlice, elem)
		}
	}

	return mergedSlice
}

func AreIntSlicesEqual(slice1, slice2 []int) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	// Create copies of the slices to avoid modifying the original slices
	s1 := append([]int(nil), slice1...)
	s2 := append([]int(nil), slice2...)

	// Sort the slices
	sort.Ints(s1)
	sort.Ints(s2)

	// Compare the sorted slices
	for i := range s1 {
		if s1[i] != s2[i] {
			return false
		}
	}

	return true
}

// OldestDate returns the oldest (earliest) date among the provided dates.
func FindOldestDate(dates ...*time.Time) *time.Time {
	var oldest *time.Time
	for _, date := range dates {
		if date == nil {
			continue
		}
		if oldest == nil || date.Before(*oldest) {
			oldest = date
		}
	}
	return oldest
}

// NormalizeDate sets the time components (hour, minute, second, nanosecond) to zero.
// func NormalizeDate(t time.Time) time.Time {
// 	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
// }

func ConvertToDate(t time.Time, timezone string) (time.Time, error) {
	if timezone == "" {
		timezone = "Asia/Yangon"
	}

	// Load the location for the given timezone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Println("Error loading location:", err)
		return t, err
	}
	localTime := t.In(location)

	// Extract only the date (without time) by using localTime.Year, Month, Day
	// We then create a new time.Time object with zero time.
	dateOnly := time.Date(localTime.Year(), localTime.Month(), localTime.Day(), 0, 0, 0, 0, location)
	return dateOnly, nil
}

// ParseDecimal converts a string to a decimal.Decimal value.
func ParseDecimal(value string) (decimal.Decimal, error) {
	// Remove any whitespace and check for empty strings
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, errors.New("empty decimal string")
	}

	// Convert string to decimal
	dec, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, err
	}

	return dec, nil
}

func BusinessLock(ctx context.Context, businessId string, lockType string, moduleName string, functionName string) error {
	logger := config.GetLogger()
	locker := config.GetRedisLock()
	if locker == nil {
		// Avoid nil-pointer panics when Redis lock isn't initialized yet.
		config.LogError(logger, moduleName, functionName, "Redis lock not initialized", businessId, errors.New("redis lock is nil"))
		return errors.New("service not ready (redis lock not initialized)")
	}
	// Try to obtain a lock for the businessID
	lockKey := fmt.Sprintf("%s:%s", lockType, businessId)
	lock, err := locker.Obtain(ctx, lockKey, 30*time.Second, nil)
	if err == redislock.ErrNotObtained {
		// Handle the case where the lock could not be obtained
		config.LogError(logger, moduleName, functionName, "Could not obtain lock for businessID", businessId, err)
		return errors.New("could not obtain lock for businessID")
	} else if err != nil {
		// Handle other errors in obtaining the lock
		config.LogError(logger, moduleName, functionName, "Error obtaining lock for businessID", businessId, err)
		return err
	}
	defer func() {
		_ = lock.Release(ctx)
	}()

	return nil

}
