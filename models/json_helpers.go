package models

import (
	"encoding/json"
	"gorm.io/datatypes"
)

func ConvertToJSON(data interface{}) (datatypes.JSON, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(bytes), nil
}