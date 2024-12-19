package mail_attach

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"r3/cache"
	"r3/data"
	"r3/db"
	"r3/schema"
	"r3/tools"
	"r3/types"

	"github.com/gofrs/uuid"
)

func DoAll() error {
	mails := make([]types.Mail, 0)

	rows, err := db.Pool.Query(db.Ctx, `
		SELECT id, record_id_wofk, attribute_id
		FROM instance.mail_spool
		WHERE outgoing = FALSE
		AND record_id_wofk IS NOT NULL
		AND attribute_id   IS NOT NULL
	`)
	if err != nil {
		return err
	}

	for rows.Next() {
		var m types.Mail

		if err := rows.Scan(&m.Id, &m.RecordId, &m.AttributeId); err != nil {
			return err
		}
		mails = append(mails, m)
	}
	rows.Close()

	for _, m := range mails {
		if err := do(m); err != nil {
			return err
		}
	}
	return nil
}

func do(mail types.Mail) error {
	cache.Schema_mx.RLock()
	defer cache.Schema_mx.RUnlock()

	// check validity of record and attributes to attach files to
	atr, exists := cache.AttributeIdMap[mail.AttributeId.Bytes]
	if !exists {
		return fmt.Errorf("cannot attach file(s) to unknown attribute %s",
			mail.AttributeId.Bytes)
	}

	if !schema.IsContentFiles(atr.Content) {
		return fmt.Errorf("cannot attach file(s) to non-file attribute %s",
			mail.AttributeId.Bytes)
	}

	// get files from spooler
	fileIds := make([]uuid.UUID, 0)
	filesMail := make([]types.MailFile, 0)

	rows, err := db.Pool.Query(db.Ctx, `
		SELECT file, file_name, file_size
		FROM instance.mail_spool_file
		WHERE mail_id = $1
	`, mail.Id)
	if err != nil {
		return err
	}

	for rows.Next() {
		var f types.MailFile

		f.Id, err = uuid.NewV4()
		if err != nil {
			return err
		}

		if err := rows.Scan(&f.File, &f.Name, &f.Size); err != nil {
			return err
		}
		fileIds = append(fileIds, f.Id)
		filesMail = append(filesMail, f)
	}
	rows.Close()

	// no attachments to process, just delete mail
	if len(filesMail) == 0 {
		_, err = db.Pool.Exec(db.Ctx, `
			DELETE FROM instance.mail_spool
			WHERE id = $1
		`, mail.Id)
		return err
	}

	// copy files
	for i, f := range filesMail {
		if err := tools.PathCreateIfNotExists(data.GetFilePathDir(f.Id), 0700); err != nil {
			return err
		}

		filePath := data.GetFilePathVersion(f.Id, 0)
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(file, bytes.NewReader(f.File)); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		filesMail[i].Hash, err = tools.GetFileHash(filePath)
		if err != nil {
			return err
		}
	}

	// store file changes
	// update the database only after all files have physically been saved
	tx, err := db.Pool.Begin(db.Ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(db.Ctx)

	fileIdMapChange := make(map[uuid.UUID]types.DataSetFileChange)
	rel, _ := cache.RelationIdMap[atr.RelationId]
	for _, f := range filesMail {
		if err := data.FileApplyVersion_tx(db.Ctx, tx, true, atr.Id, rel.Id,
			f.Id, f.Hash, f.Name, f.Size, 0, []int64{mail.RecordId.Int64}, -1); err != nil {

			return err
		}

		fileIdMapChange[f.Id] = types.DataSetFileChange{
			Action:  "create",
			Name:    f.Name,
			Version: -1,
		}
	}
	if err := data.FilesApplyAttributChanges_tx(db.Ctx, tx, mail.RecordId.Int64,
		atr.Id, fileIdMapChange); err != nil {
		return err
	}

	// all done, delete mail
	if _, err := tx.Exec(db.Ctx, `
		DELETE FROM instance.mail_spool
		WHERE id = $1
	`, mail.Id); err != nil {
		return err
	}
	return tx.Commit(db.Ctx)
}
