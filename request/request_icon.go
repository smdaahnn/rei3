package request

import (
	"encoding/json"
	"r3/schema/icon"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
)

func IconDel_tx(tx pgx.Tx, reqJson json.RawMessage) (interface{}, error) {
	var req struct {
		Id uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(reqJson, &req); err != nil {
		return nil, err
	}
	return nil, icon.Del_tx(tx, req.Id)
}

func IconSetName_tx(tx pgx.Tx, reqJson json.RawMessage) (interface{}, error) {
	var req struct {
		Id       uuid.UUID `json:"id"`
		ModuleId uuid.UUID `json:"moduleId"`
		Name     string    `json:"name"`
	}
	if err := json.Unmarshal(reqJson, &req); err != nil {
		return nil, err
	}
	return nil, icon.SetName_tx(tx, req.ModuleId, req.Id, req.Name)
}
