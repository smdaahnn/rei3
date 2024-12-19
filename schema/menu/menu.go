package menu

import (
	"fmt"
	"r3/db"
	"r3/schema"
	"r3/schema/caption"
	"r3/schema/collection/consumer"
	"r3/types"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func Copy_tx(tx pgx.Tx, moduleId uuid.UUID, moduleIdNew uuid.UUID) error {

	menus, err := Get(moduleId, pgtype.UUID{})
	if err != nil {
		return err
	}

	// reset entity IDs
	menus = NilIds(menus, moduleIdNew)

	return Set_tx(tx, pgtype.UUID{}, menus)
}

func Del_tx(tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(db.Ctx, `DELETE FROM app.menu WHERE id = $1`, id)
	return err
}

func Get(moduleId uuid.UUID, parentId pgtype.UUID) ([]types.Menu, error) {

	menus := make([]types.Menu, 0)

	nullCheck := "AND (parent_id IS NULL OR parent_id = $2)"
	if parentId.Valid {
		nullCheck = "AND parent_id = $2"
	}

	rows, err := db.Pool.Query(db.Ctx, fmt.Sprintf(`
		SELECT id, form_id, icon_id, show_children, color
		FROM app.menu
		WHERE module_id = $1
		%s
		ORDER BY position ASC
	`, nullCheck), moduleId, parentId)
	if err != nil {
		return menus, err
	}

	for rows.Next() {
		var m types.Menu

		if err := rows.Scan(&m.Id, &m.FormId, &m.IconId, &m.ShowChildren, &m.Color); err != nil {
			return menus, err
		}
		m.ModuleId = moduleId
		menus = append(menus, m)
	}
	rows.Close()

	for i, m := range menus {

		// get children & collections & captions
		m.Menus, err = Get(moduleId, pgtype.UUID{Bytes: m.Id, Valid: true})
		if err != nil {
			return menus, err
		}
		m.Collections, err = consumer.Get("menu", m.Id, "menuDisplay")
		if err != nil {
			return menus, err
		}
		m.Captions, err = caption.Get("menu", m.Id, []string{"menuTitle"})
		if err != nil {
			return menus, err
		}
		menus[i] = m
	}
	return menus, nil
}

func Set_tx(tx pgx.Tx, parentId pgtype.UUID, menus []types.Menu) error {

	for i, m := range menus {
		known, err := schema.CheckCreateId_tx(tx, &m.Id, "menu", "id")
		if err != nil {
			return err
		}

		if known {
			if _, err := tx.Exec(db.Ctx, `
				UPDATE app.menu
				SET parent_id = $1, form_id = $2, icon_id = $3, position = $4,
					show_children = $5, color = $6
				WHERE id = $7
			`, parentId, m.FormId, m.IconId, i, m.ShowChildren, m.Color, m.Id); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(db.Ctx, `
				INSERT INTO app.menu (id, module_id, parent_id, form_id,
					icon_id, position, show_children, color)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			`, m.Id, m.ModuleId, parentId, m.FormId, m.IconId, i, m.ShowChildren, m.Color); err != nil {
				return err
			}
		}

		// set children
		if err := Set_tx(tx, pgtype.UUID{Bytes: m.Id, Valid: true}, m.Menus); err != nil {
			return err
		}

		// set collections
		if err := consumer.Set_tx(tx, "menu", m.Id, "menuDisplay", m.Collections); err != nil {
			return err
		}

		// set captions
		if err := caption.Set_tx(tx, m.Id, m.Captions); err != nil {
			return err
		}
	}
	return nil
}

// nil menu IDs and set new module
func NilIds(menus []types.Menu, moduleIdNew uuid.UUID) []types.Menu {

	for i, _ := range menus {
		menus[i].Id = uuid.Nil
		menus[i].ModuleId = moduleIdNew

		for j, _ := range menus[i].Collections {
			menus[i].Collections[j].Id = uuid.Nil
		}
		menus[i].Menus = NilIds(menus[i].Menus, moduleIdNew)
	}
	return menus
}
