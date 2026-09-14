package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultListingStatus = "ready"
)

const maxCityLength = 100

type listingFilters struct {
	Status   string
	City     string
	MinPrice *int64
	MaxPrice *int64
}

// zero value - 0 -> nil if MinPrice == nil {}
// 0 - 1000000
// status=ready&city=delhi&min_price=1000000&max_price=2000000
func parseListingsFilters(params url.Values) (listingFilters, error) {
	var f listingFilters
	f.Status = defaultListingStatus

	city := strings.TrimSpace(params.Get("city"))
	if len(city) > maxCityLength {
		return listingFilters{}, &ValidationError{Field: "city", Msg: fmt.Sprintf("must be at most %d chars", maxCityLength)}
	}
	f.City = city

	minPrice, err := parsePrice(params.Get("min_price"), "min_price")
	if err != nil {
		return listingFilters{}, err
	}

	maxPrice, err := parsePrice(params.Get("max_price"), "max_price")
	if err != nil {
		return listingFilters{}, err
	}

	f.MinPrice = minPrice
	f.MaxPrice = maxPrice

	return f, nil
}

// "1000000"
func parsePrice(rawPrice, field string) (*int64, error) {
	if rawPrice == "" {
		return nil, nil
	}

	n, err := strconv.ParseInt(rawPrice, 10, 64)
	if err != nil {
		return nil, &ValidationError{Field: field, Msg: "must be an integer"}
	}

	if n < 0 {
		return nil, &ValidationError{Field: field, Msg: "must not be negetive"}
	}

	return &n, nil
}

// l2.status = "ready"
// 		AND l2.city = $1
// 	  AND l2.price >= $2
// 	  AND l2.price <= $3
//    AND (l2.created_at, l2.id) < ($4, $5)

const queryTemplate = `SELECT l.id, l.title, l.description, l.price, l.city, l.status, l.created_at, l.user_id, i.id as image_id, i.object_key, i.position
FROM listings l
LEFT JOIN images i ON i.listing_id = l.id
WHERE l.id IN (
	SELECT id FROM listings l2
	WHERE %s
	ORDER BY l2.created_at DESC, l2.id DESC
	LIMIT $%d
	)
ORDER BY l.created_at DESC, l.id DESC, i.position`

func buildListingQuery(filters listingFilters, cur *listingCursor, limit int) (string, []any) {

	where := []string{fmt.Sprintf("l2.status = '%s'", filters.Status)}
	args := []any{}

	if filters.City != "" {
		args = append(args, strings.ToLower(filters.City))
		where = append(where, fmt.Sprintf("lower(l2.city) = $%d", len(args))) // 1  city = $1
	}

	if filters.MinPrice != nil {
		args = append(args, *filters.MinPrice)
		where = append(where, fmt.Sprintf("l2.price >= $%d", len(args)))
	}

	if filters.MaxPrice != nil {
		args = append(args, *filters.MaxPrice)
		where = append(where, fmt.Sprintf("l2.price <= $%d", len(args)))
	}

	if cur != nil {
		args = append(args, cur.CreatedAt, cur.ID)
		where = append(where, fmt.Sprintf("(l2.created_at, l2.id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	args = append(args, limit)

	query := fmt.Sprintf(
		queryTemplate,
		strings.Join(where, "\n\t\t AND "),
		len(args),
	)

	return query, args
}
