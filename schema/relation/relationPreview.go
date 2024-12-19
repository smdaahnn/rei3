package relation

import (
	"fmt"
	"r3/db"
	"r3/schema"
	"strings"

	"github.com/gofrs/uuid"
)

func GetPreview(id uuid.UUID, limit int, offset int) (interface{}, error) {

	var modName, relName string
	atrNames := make([]string, 0)

	res := struct {
		Rows     []interface{} `json:"rows"`
		RowCount int64         `json:"rowCount"`
	}{
		make([]interface{}, 0),
		0,
	}

	// get relation/attribute/module details
	if err := db.Pool.QueryRow(db.Ctx, `
		SELECT r.name, m.name, ARRAY(
			SELECT name
			FROM app.attribute
			WHERE relation_id =  r.id
			AND   content     <> 'files'
			ORDER BY CASE WHEN name = 'id' THEN 0 END, name ASC
		) AS atrs
		FROM app.relation AS r
		INNER JOIN app.module AS m ON m.id = r.module_id
		WHERE r.id = $1
	`, id).Scan(&relName, &modName, &atrNames); err != nil {
		return nil, err
	}

	// get total count of tupels from relation
	if err := db.Pool.QueryRow(db.Ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM "%s"."%s"
	`, modName, relName)).Scan(&res.RowCount); err != nil {
		return nil, err
	}

	// get records from relation
	rows, err := db.Pool.Query(db.Ctx, fmt.Sprintf(`
		SELECT "%s"
		FROM "%s"."%s"
		ORDER BY "%s" ASC
		LIMIT $1
		OFFSET $2
	`, strings.Join(atrNames, `", "`), modName, relName, schema.PkName), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		valuesAll, err := rows.Values()
		if err != nil {
			return nil, err
		}
		res.Rows = append(res.Rows, valuesAll)
	}
	return res, nil
}
