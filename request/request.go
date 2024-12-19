package request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"r3/cache"
	"r3/cluster"
	"r3/config"
	"r3/db"
	"r3/handler"
	"r3/log"
	"r3/types"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// executes a websocket transaction with multiple requests within a single DB transaction
func ExecTransaction(ctxClient context.Context, address string, loginId int64, isAdmin bool,
	device types.WebsocketClientDevice, isNoAuth bool, reqTrans types.RequestTransaction,
	resTrans types.ResponseTransaction) types.ResponseTransaction {

	// start transaction
	ctx, ctxCancel := context.WithTimeout(ctxClient,
		time.Duration(int64(config.GetUint64("dbTimeoutDataWs")))*time.Second)

	defer ctxCancel()

	// run in a loop as there is an error case where it needs to be repeated
	runAgainNewCache := false
	for runOnce := true; runOnce || runAgainNewCache; runOnce = false {

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			log.Error("websocket", "cannot begin transaction", err)
			resTrans.Error = handler.ErrGeneral
			return resTrans
		}

		if runAgainNewCache {
			if err := tx.Conn().DeallocateAll(ctx); err != nil {
				log.Error("websocket", "failed to deallocate DB connection", err)
				resTrans.Error = handler.ErrGeneral
				return resTrans
			}
			runAgainNewCache = false
			resTrans.Error = ""
		}

		// set local transaction configuration parameters
		// these are used by system functions, such as instance.get_login_id()
		if _, err := tx.Exec(ctx, `SELECT SET_CONFIG('r3.login_id',$1,TRUE)`, strconv.FormatInt(loginId, 10)); err != nil {

			log.Error("websocket", fmt.Sprintf("TRANSACTION %d, transaction config failure (login ID %d)",
				reqTrans.TransactionNr, loginId), err)

			return resTrans
		}

		// work through requests
		for _, req := range reqTrans.Requests {

			log.Info("websocket", fmt.Sprintf("TRANSACTION %d, %s %s, payload: %s",
				reqTrans.TransactionNr, req.Action, req.Ressource, req.Payload))

			payload, err := Exec_tx(ctx, tx, address, loginId, isAdmin,
				device, isNoAuth, req.Ressource, req.Action, req.Payload)

			if err == nil {
				// all clear, prepare response payload
				var res types.Response
				res.Payload, err = json.Marshal(payload)
				if err == nil {
					resTrans.Responses = append(resTrans.Responses, res)
					continue
				}
			}

			// error case, convert to error code for requestor
			returnErr, isExpectedErr := handler.ConvertToErrCode(err, !isAdmin)
			if !isExpectedErr {
				log.Warning("websocket", fmt.Sprintf("TRANSACTION %d, request %s %s failure (login ID %d)",
					reqTrans.TransactionNr, req.Ressource, req.Action, loginId), err)
			}

			if handler.CheckForDbsCacheErrCode(returnErr) {
				runAgainNewCache = true
			}
			resTrans.Error = fmt.Sprintf("%v", returnErr)
			break
		}

		// check if error occured in any request
		if resTrans.Error == "" {
			if err := tx.Commit(ctx); err != nil {

				returnErr, isExpectedErr := handler.ConvertToErrCode(err, !isAdmin)
				if !isExpectedErr {
					log.Warning("websocket", fmt.Sprintf("TRANSACTION %d, commit failure (login ID %d)",
						reqTrans.TransactionNr, loginId), err)
				}
				resTrans.Error = fmt.Sprintf("%v", returnErr)
				resTrans.Responses = make([]types.Response, 0)
				tx.Rollback(ctx)
			}
		} else {
			resTrans.Responses = make([]types.Response, 0)
			tx.Rollback(ctx)
		}
	}
	return resTrans
}

func Exec_tx(ctx context.Context, tx pgx.Tx, address string, loginId int64, isAdmin bool,
	device types.WebsocketClientDevice, isNoAuth bool, ressource string, action string,
	reqJson json.RawMessage) (interface{}, error) {

	// public requests: accessible to all
	switch ressource {
	case "public":
		switch action {
		case "get":
			return PublicGet()
		}
	}

	if loginId == 0 {
		return nil, errors.New(handler.ErrUnauthorized)
	}

	// authorized requests: fat-client
	if device == types.WebsocketClientDeviceFatClient {
		switch ressource {
		case "clientApp":
			switch action {
			case "getBuild": // current client app build
				return config.GetAppVersionClient().Build, nil
			}
		case "clientEvent":
			switch action {
			case "exec":
				return clientEventExecFatClient(reqJson, loginId, address)
			case "get":
				return clientEventGetFatClient(loginId)
			}
		}
		return nil, errors.New(handler.ErrUnauthorized)
	}

	// authorized requests: non-admin
	switch ressource {
	case "data":
		switch action {
		case "del":
			return DataDel_tx(ctx, tx, reqJson, loginId)
		case "get":
			return DataGet_tx(ctx, tx, reqJson, loginId)
		case "getKeys":
			return DataGetKeys_tx(ctx, tx, reqJson, loginId)
		case "getLog":
			return DataLogGet_tx(ctx, tx, reqJson, loginId)
		case "set":
			return DataSet_tx(ctx, tx, reqJson, loginId)
		case "setKeys":
			return DataSetKeys_tx(ctx, tx, reqJson)
		}
	case "event":
		switch action {
		case "clientEventsChanged":
			return eventClientEventsChanged(loginId, address)
		case "filesCopied":
			return eventFilesCopied(reqJson, loginId, address)
		case "fileRequested":
			return eventFileRequested(reqJson, loginId, address)
		case "keystrokesRequested":
			return eventKeystrokesRequested(reqJson, loginId, address)
		}
	case "feedback":
		switch action {
		case "send":
			return FeedbackSend_tx(tx, reqJson)
		}
	case "file":
		switch action {
		case "paste":
			return filesPaste(reqJson, loginId)
		}
	case "login":
		switch action {
		case "getNames":
			return LoginGetNames(reqJson)
		case "delTokenFixed":
			return LoginDelTokenFixed(reqJson, loginId)
		case "getTokensFixed":
			return LoginGetTokensFixed(loginId)
		case "setTokenFixed":
			return LoginSetTokenFixed_tx(tx, reqJson, loginId)
		}
	case "loginClientEvent":
		switch action {
		case "del":
			return loginClientEventDel_tx(tx, reqJson, loginId)
		case "get":
			return loginClientEventGet(loginId)
		case "set":
			return loginClientEventSet_tx(tx, reqJson, loginId)
		}
	case "loginKeys":
		switch action {
		case "getPublic":
			return LoginKeysGetPublic(ctx, reqJson)
		case "reset":
			return LoginKeysReset_tx(tx, loginId)
		case "store":
			return LoginKeysStore_tx(tx, reqJson, loginId)
		case "storePrivate":
			return LoginKeysStorePrivate_tx(tx, reqJson, loginId)
		}
	case "loginPassword":
		switch action {
		case "set":
			if isNoAuth {
				return nil, errors.New(handler.ErrUnauthorized)
			}
			return loginPasswortSet_tx(tx, reqJson, loginId)
		}
	case "loginSetting":
		switch action {
		case "get":
			return LoginSettingsGet(loginId)
		case "set":
			if isNoAuth {
				return nil, errors.New(handler.ErrUnauthorized)
			}
			return LoginSettingsSet_tx(tx, reqJson, loginId)
		}
	case "loginWidgetGroups":
		switch action {
		case "get":
			return LoginWidgetGroupsGet(loginId)
		case "set":
			return LoginWidgetGroupsSet_tx(tx, reqJson, loginId)
		}
	case "lookup":
		switch action {
		case "get":
			return lookupGet(reqJson, loginId)
		}
	case "pgFunction":
		switch action {
		case "exec": // user may exec non-trigger backend function, available to frontend
			return PgFunctionExec_tx(tx, reqJson, true)
		}
	}

	// authorized requests: admin
	if !isAdmin {
		return nil, errors.New(handler.ErrUnauthorized)
	}

	switch ressource {
	case "api":
		switch action {
		case "copy":
			return ApiCopy_tx(tx, reqJson)
		case "del":
			return ApiDel_tx(tx, reqJson)
		case "set":
			return ApiSet_tx(tx, reqJson)
		}
	case "article":
		switch action {
		case "assign":
			return ArticleAssign_tx(tx, reqJson)
		case "del":
			return ArticleDel_tx(tx, reqJson)
		case "set":
			return ArticleSet_tx(tx, reqJson)
		}
	case "attribute":
		switch action {
		case "del":
			return AttributeDel_tx(tx, reqJson)
		case "delCheck":
			return AttributeDelCheck_tx(tx, reqJson)
		case "set":
			return AttributeSet_tx(tx, reqJson)
		}
	case "backup":
		switch action {
		case "get":
			return BackupGet()
		}
	case "bruteforce":
		switch action {
		case "get":
			return BruteforceGet(reqJson)
		}
	case "captionMap":
		switch action {
		case "get":
			return CaptionMapGet(reqJson)
		case "setOne":
			return CaptionMapSetOne_tx(tx, reqJson)
		}
	case "clientEvent":
		switch action {
		case "del":
			return clientEventDel_tx(tx, reqJson)
		case "set":
			return clientEventSet_tx(tx, reqJson)
		}
	case "collection":
		switch action {
		case "del":
			return CollectionDel_tx(tx, reqJson)
		case "set":
			return CollectionSet_tx(tx, reqJson)
		}
	case "config":
		switch action {
		case "get":
			return ConfigGet()
		case "set":
			return ConfigSet_tx(tx, reqJson)
		}
	case "cluster":
		switch action {
		case "delNode":
			return ClusterNodeDel_tx(tx, reqJson)
		case "getNodes":
			return ClusterNodesGet()
		case "setNode":
			return ClusterNodeSet_tx(tx, reqJson)
		case "shutdownNode":
			return ClusterNodeShutdown(reqJson)
		}
	case "dataSql":
		switch action {
		case "get":
			return DataSqlGet_tx(ctx, tx, reqJson, loginId)
		}
	case "field":
		switch action {
		case "del":
			return FieldDel_tx(tx, reqJson)
		}
	case "file":
		switch action {
		case "get":
			return FileGet()
		case "restore":
			return FileRestore(reqJson)
		}
	case "form":
		switch action {
		case "copy":
			return FormCopy_tx(tx, reqJson)
		case "del":
			return FormDel_tx(tx, reqJson)
		case "set":
			return FormSet_tx(tx, reqJson)
		}
	case "icon":
		switch action {
		case "del":
			return IconDel_tx(tx, reqJson)
		case "setName":
			return IconSetName_tx(tx, reqJson)
		}
	case "jsFunction":
		switch action {
		case "del":
			return JsFunctionDel_tx(tx, reqJson)
		case "set":
			return JsFunctionSet_tx(tx, reqJson)
		}
	case "key":
		switch action {
		case "create":
			return KeyCreate(reqJson)
		}
	case "ldap":
		switch action {
		case "check":
			return LdapCheck(reqJson)
		case "del":
			return LdapDel_tx(tx, reqJson)
		case "get":
			return LdapGet()
		case "import":
			return LdapImport(reqJson)
		case "reload":
			return nil, cache.LoadLdapMap()
		case "set":
			return LdapSet_tx(tx, reqJson)
		}
	case "license":
		switch action {
		case "del":
			return  LicenseDel_tx(tx)
		case "get":
			return config.GetLicense(), nil
		}
	case "log":
		switch action {
		case "get":
			return LogGet(reqJson)
		}
	case "login":
		switch action {
		case "del":
			return LoginDel_tx(tx, reqJson)
		case "get":
			return LoginGet(reqJson)
		case "getConcurrent":
			return LoginGetConcurrent()
		case "getMembers":
			return LoginGetMembers(reqJson)
		case "getRecords":
			return LoginGetRecords(reqJson)
		case "kick":
			return LoginKick(reqJson)
		case "reauth":
			return LoginReauth(reqJson)
		case "reauthAll":
			return LoginReauthAll()
		case "resetTotp":
			return LoginResetTotp_tx(tx, reqJson)
		case "set":
			return LoginSet_tx(tx, reqJson)
		case "setMembers":
			return LoginSetMembers_tx(tx, reqJson)
		}
	case "loginForm":
		switch action {
		case "del":
			return LoginFormDel_tx(tx, reqJson)
		case "set":
			return LoginFormSet_tx(tx, reqJson)
		}
	case "loginTemplate":
		switch action {
		case "del":
			return LoginTemplateDel_tx(tx, reqJson)
		case "get":
			return LoginTemplateGet(reqJson)
		case "set":
			return LoginTemplateSet_tx(tx, reqJson)
		}
	case "mailAccount":
		switch action {
		case "del":
			return MailAccountDel_tx(tx, reqJson)
		case "get":
			return MailAccountGet()
		case "reload":
			return MailAccountReload()
		case "set":
			return MailAccountSet_tx(tx, reqJson)
		case "test":
			return MailAccountTest_tx(tx, reqJson)
		}
	case "mailSpooler":
		switch action {
		case "del":
			return MailSpoolerDel_tx(tx, reqJson)
		case "get":
			return MailSpoolerGet(reqJson)
		case "reset":
			return MailSpoolerReset_tx(tx, reqJson)
		}
	case "mailTraffic":
		switch action {
		case "get":
			return MailTrafficGet(reqJson)
		}
	case "menu":
		switch action {
		case "copy":
			return MenuCopy_tx(tx, reqJson)
		case "del":
			return MenuDel_tx(tx, reqJson)
		case "set":
			return MenuSet_tx(tx, reqJson)
		}
	case "module":
		switch action {
		case "checkChange":
			return ModuleCheckChange_tx(tx, reqJson)
		case "del":
			return ModuleDel_tx(tx, reqJson)
		case "set":
			return ModuleSet_tx(tx, reqJson)
		}
	case "moduleMeta":
		switch action {
		case "setLanguagesCustom":
			return ModuleMetaSetLanguagesCustom_tx(tx, reqJson)
		case "setOptions":
			return ModuleMetaSetOptions_tx(tx, reqJson)
		}
	case "oauthClient":
		switch action {
		case "del":
			return OauthClientDel_tx(tx, reqJson)
		case "get":
			return OauthClientGet()
		case "reload":
			return OauthClientReload()
		case "set":
			return OauthClientSet_tx(tx, reqJson)
		}
	case "package":
		switch action {
		case "install":
			return PackageInstall()
		}
	case "pgFunction":
		switch action {
		case "del":
			return PgFunctionDel_tx(tx, reqJson)
		case "execAny": // admin may exec any non-trigger backend function
			return PgFunctionExec_tx(tx, reqJson, false)
		case "set":
			return PgFunctionSet_tx(tx, reqJson)
		}
	case "pgIndex":
		switch action {
		case "del":
			return PgIndexDel_tx(tx, reqJson)
		case "get":
			return PgIndexGet(reqJson)
		case "set":
			return PgIndexSet_tx(tx, reqJson)
		}
	case "pgTrigger":
		switch action {
		case "del":
			return PgTriggerDel_tx(tx, reqJson)
		case "set":
			return PgTriggerSet_tx(tx, reqJson)
		}
	case "preset":
		switch action {
		case "del":
			return PresetDel_tx(tx, reqJson)
		case "set":
			return PresetSet_tx(tx, reqJson)
		}
	case "pwaDomain":
		switch action {
		case "reset":
			return nil, cache.LoadPwaDomainMap()
		case "set":
			return PwaDomainSet_tx(tx, reqJson)
		}
	case "relation":
		switch action {
		case "del":
			return RelationDel_tx(tx, reqJson)
		case "get":
			return RelationGet(reqJson)
		case "preview":
			return RelationPreview(reqJson)
		case "set":
			return RelationSet_tx(tx, reqJson)
		}
	case "repoModule":
		switch action {
		case "get":
			return RepoModuleGet(reqJson)
		case "install":
			return RepoModuleInstall(reqJson)
		case "installAll":
			return RepoModuleInstallAll()
		case "update":
			return RepoModuleUpdate()
		}
	case "role":
		switch action {
		case "del":
			return RoleDel_tx(tx, reqJson)
		case "set":
			return RoleSet_tx(tx, reqJson)
		}
	case "scheduler":
		switch action {
		case "get":
			return schedulersGet()
		}
	case "schema":
		switch action {
		case "check":
			return SchemaCheck_tx(tx, reqJson)
		case "reload":
			return SchemaReload(reqJson)
		}
	case "system":
		switch action {
		case "get":
			return SystemGet()
		}
	case "task":
		switch action {
		case "informChanged":
			return nil, cluster.TasksChanged(true)
		case "run":
			return TaskRun(reqJson)
		case "set":
			return TaskSet_tx(tx, reqJson)
		}
	case "transfer":
		switch action {
		case "addVersion":
			return TransferAddVersion_tx(tx, reqJson)
		case "storeExportKey":
			return TransferStoreExportKey(reqJson)
		}
	case "widget":
		switch action {
		case "del":
			return WidgetDel_tx(tx, reqJson)
		case "set":
			return WidgetSet_tx(tx, reqJson)
		}
	}
	return nil, fmt.Errorf("unknown ressource or action")
}
