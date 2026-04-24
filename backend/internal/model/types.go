package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap GORM 自定义类型，实现 Scanner/Valuer
type JSONMap map[string]any

// GormDataType 返回 GORM 列类型
func (JSONMap) GormDataType() string {
	return "TEXT"
}

// Scan 实现 sql.Scanner 接口
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSONMap: %v", value)
	}
	return json.Unmarshal(bytes, j)
}

// Value 实现 driver.Valuer 接口
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}
