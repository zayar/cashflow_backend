package models

import (
	"fmt"

	"bitbucket.org/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// new pagination combined struct embedding + generic struct
type Cursor interface {
	GetCursor() string
}

type Edge[N Cursor] struct {
	Node   *N
	Cursor string
}

// fetch results for pagination
func FetchPagePureCursor[T Cursor](dbCtx *gorm.DB,
	limit int,
	after *string,
	cursorColumn string,
	cmpOperator string,
) ([]Edge[T], *PageInfo, error) {

	nodes := make([]*T, 0)

	// order
	if cmpOperator == ">" {
		dbCtx.Order(cursorColumn)
	} else if cmpOperator == "<" {
		dbCtx.Order(cursorColumn + " DESC")
	}

	// filter
	decodedCursor, err := DecodeCursor(after)
	if err != nil {
		return nil, nil, err
	}
	if decodedCursor != "" {
		dbCtx.Where(cursorColumn+" "+cmpOperator+" ?", decodedCursor)
	}

	// db query
	dbCtx.Limit(limit + 1)
	if err = dbCtx.Find(&nodes).Error; err != nil {
		return nil, nil, err
	}

	/*
		constructing edges & page info
	*/
	count := 0
	hasNextPage := false
	edges := make([]Edge[T], 0, len(nodes))
	for _, node := range nodes {
		if count == limit {
			hasNextPage = true
		}
		if count < limit {
			var edge Edge[T]
			edge.Node = node
			edge.Cursor = EncodeCursor((*node).GetCursor())
			edges = append(edges, edge)
			count++
		}
	}

	pageInfo := PageInfo{
		StartCursor: "",
		EndCursor:   "",
		HasNextPage: utils.NewFalse(),
	}
	if count > 0 {
		pageInfo = PageInfo{
			StartCursor: edges[0].Cursor,
			EndCursor:   edges[count-1].Cursor,
			HasNextPage: &hasNextPage,
		}
	}

	return edges, &pageInfo, nil
}

type CompositeCursor interface {
	Cursor
	Identifier
}

// fetch results for pagination
func FetchPageCompositeCursor[T CompositeCursor](dbCtx *gorm.DB,
	limit int,
	after *string,
	cursorColumn string,
	cmpOperator string,
) ([]Edge[T], *PageInfo, error) {

	nodes := make([]*T, 0)

	// order
	if cmpOperator == ">" {
		dbCtx.Order(cursorColumn + ", id")
	} else if cmpOperator == "<" {
		dbCtx.Order(cursorColumn + " DESC, id DESC")
	}

	// filter
	decodedCursor, cursorId := DecodeCompositeCursor(after)
	if decodedCursor != "" {
		dbCtx.Where(
			// [1] = column, [2] = operator
			fmt.Sprintf("%[1]s %[2]s ? OR (%[1]s = ? AND id %[2]s ?)", cursorColumn, cmpOperator),
			decodedCursor, decodedCursor, cursorId)
	}

	// db query
	dbCtx.Limit(limit + 1)
	if err := dbCtx.Find(&nodes).Error; err != nil {
		return nil, nil, err
	}

	/*
		constructing edges & page info
	*/
	count := 0
	hasNextPage := false
	edges := make([]Edge[T], 0, len(nodes))
	for _, node := range nodes {
		if count == limit {
			hasNextPage = true
		}
		if count < limit {
			var edge Edge[T]
			edge.Node = node
			edge.Cursor = EncodeCompositeCursor((*node).GetCursor(), (*node).GetId())
			edges = append(edges, edge)
			count++
		}
	}

	pageInfo := PageInfo{
		StartCursor: "",
		EndCursor:   "",
		HasNextPage: utils.NewFalse(),
	}
	if count > 0 {
		pageInfo = PageInfo{
			StartCursor: edges[0].Cursor,
			EndCursor:   edges[count-1].Cursor,
			HasNextPage: &hasNextPage,
		}
	}

	return edges, &pageInfo, nil
}

// func ConnectNodes[T Node](nodes []*T, limit int) ([]Edge[T], *PageInfo) {
// 	count := 0
// 	hasNextPage := false
// 	edges := make([]Edge[T], 0, len(nodes))
// 	for _, node := range nodes {
// 		if count == limit {
// 			hasNextPage = true
// 		}
// 		if count < limit {
// 			var edge Edge[T]
// 			edge.Node = node
// 			edge.Cursor = (*node).GetCursor()
// 			edges = append(edges, edge)
// 			count++
// 		}
// 	}
// 	var pageInfo PageInfo
// 	if count == 0 {
// 		pageInfo = PageInfo{
// 			StartCursor: "",
// 			EndCursor:   "",
// 			HasNextPage: utils.NewFalse(),
// 		}
// 	} else {
// 		pageInfo = PageInfo{
// 			StartCursor: EncodeCursor((*nodes[0]).GetCursor()),
// 			EndCursor:   EncodeCursor((*nodes[count-1]).GetCursor()),
// 			HasNextPage: &hasNextPage,
// 		}
// 	}

// 	return edges, &pageInfo
// }

// func PaginateOnlyCursor[N Node]() (conn *Connection[N]) {
// 	var nodes []N
// 	for _, node := range nodes {
// 		var edge Edge[N]
// 		edge.Node = &node
// 		conn.Edges = append(conn.Edges, &edge)
// 	}
// 	return
// }

// func (c *Connection[E, N]) AddEdge(node N) {
// 	var edge E
// 	edge.SetNode(node)
// 	// edge.SetNode(node)
// 	// c.Edges = append(c.Edges, &edge)
// }
