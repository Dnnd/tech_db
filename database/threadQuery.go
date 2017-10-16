package database

import "bytes"

type ThreadQueryBuilder struct {
	Query bytes.Buffer
}

func NewThreadQueryBuilder() *ThreadQueryBuilder{
	return &ThreadQueryBuilder{}
}

