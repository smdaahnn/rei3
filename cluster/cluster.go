package cluster

import (
	"encoding/json"
	"r3/cache"
	"r3/config"
	"r3/db"
	"r3/log"
	"r3/tools"
	"r3/types"
	"sync/atomic"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	SchedulerRestart      = make(chan bool, 10)
	websocketClientCount  atomic.Int32
	WebsocketClientEvents = make(chan types.ClusterEvent, 10)
)

func SetWebsocketClientCount(value int) {
	websocketClientCount.Store(int32(value))
}

// register cluster node with shared database
// read existing node ID from configuration file if exists
func StartNode() error {

	// create node ID for itself if it does not exist yet
	if config.File.Cluster.NodeId == "" {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}

		// write node ID to local config file
		config.File.Cluster.NodeId = id.String()

		if err := config.WriteFile(); err != nil {
			return err
		}
	}

	// read node ID from config file
	nodeId, err := uuid.FromString(config.File.Cluster.NodeId)
	if err != nil {
		return err
	}

	// check whether node is already registered
	var nodeName string
	err = db.Pool.QueryRow(db.Ctx, `
		SELECT name
		FROM instance_cluster.node
		WHERE id = $1
	`, nodeId).Scan(&nodeName)

	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	exists := err != pgx.ErrNoRows

	if !exists {
		// generate new node name
		if err := db.Pool.QueryRow(db.Ctx, `
			SELECT CONCAT('node',(COUNT(*)+1)::TEXT)
			FROM instance_cluster.node
		`).Scan(&nodeName); err != nil {
			return err
		}

		if _, err := db.Pool.Exec(db.Ctx, `
			INSERT INTO instance_cluster.node (id,name,hostname,date_started,
				date_check_in,stat_sessions,stat_memory,cluster_master,running)
			VALUES ($1,$2,$3,$4,0,-1,-1,false,true)
		`, nodeId, nodeName, cache.GetHostname(), tools.GetTimeUnix()); err != nil {
			return err
		}
	} else {
		// node is starting up - set start time, disable master role and delete missed events
		if _, err := db.Pool.Exec(db.Ctx, `
			UPDATE instance_cluster.node
			SET date_started = $1, cluster_master = false, running = true
			WHERE id = $2
		`, tools.GetTimeUnix(), nodeId); err != nil {
			return err
		}

		if _, err := db.Pool.Exec(db.Ctx, `
			DELETE FROM instance_cluster.node_event
			WHERE node_id = $1
		`, nodeId); err != nil {
			return err
		}
	}

	// store node details
	cache.SetNodeId(nodeId)
	cache.SetNodeName(nodeName)
	log.SetNodeId(nodeId)
	return nil
}
func StopNode() error {
	// on shutdown: Give up master role and disable running state
	_, err := db.Pool.Exec(db.Ctx, `
		UPDATE instance_cluster.node
		SET cluster_master = false, running = false
		WHERE id = $1
	`, cache.GetNodeId())
	return err
}
func DelNode_tx(tx pgx.Tx, id uuid.UUID) error {
	_, err := db.Pool.Exec(db.Ctx, `
		DELETE FROM instance_cluster.node
		WHERE id = $1
	`, id)
	return err
}
func GetNodes() ([]types.ClusterNode, error) {
	nodes := make([]types.ClusterNode, 0)

	rows, err := db.Pool.Query(db.Ctx, `
		SELECT id, name, hostname, cluster_master, running,
			date_check_in, date_started, stat_memory, stat_sessions
		FROM instance_cluster.node
		ORDER BY name
	`)
	if err != nil {
		return nodes, err
	}
	defer rows.Close()

	for rows.Next() {
		var n types.ClusterNode

		if err := rows.Scan(&n.Id, &n.Name, &n.Hostname, &n.ClusterMaster,
			&n.Running, &n.DateCheckIn, &n.DateStarted, &n.StatMemory,
			&n.StatSessions); err != nil {

			return nodes, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}
func SetNode_tx(tx pgx.Tx, id uuid.UUID, name string) error {
	_, err := db.Pool.Exec(db.Ctx, `
		UPDATE instance_cluster.node
		SET name = $1
		WHERE id = $2
	`, name, id)
	return err
}

// helper
// creates node events to some nodes (by node IDs) or all but the current node (if no node IDs are given)
func CreateEventForNodes(nodeIds []uuid.UUID, content string, payload interface{}, target types.ClusterEventTarget) error {
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	address := pgtype.Text{
		String: target.Address,
		Valid:  target.Address != "",
	}
	device := pgtype.Int2{
		Int16: int16(target.Device),
		Valid: target.Device != 0,
	}
	loginId := pgtype.Int8{
		Int64: target.LoginId,
		Valid: target.LoginId != 0,
	}

	// only generate events for nodes that have checked in within the last hour
	// node events are temporary and not relevant for nodes checking in after the fact
	checkInCutOff := tools.GetTimeUnix() - 3600

	if len(nodeIds) == 0 {
		// if no node IDs are defined, apply to all other nodes
		if _, err := db.Pool.Exec(db.Ctx, `
			INSERT INTO instance_cluster.node_event (
				node_id, content, payload, target_address,
				target_device, target_login_id
			)
			SELECT id, $1, $2, $3, $4, $5
			FROM instance_cluster.node
			WHERE id            <> $6
			AND   date_check_in >  $7
		`, content, payloadJson, address, device, loginId, cache.GetNodeId(), checkInCutOff); err != nil {
			return err
		}
	} else {
		if _, err := db.Pool.Exec(db.Ctx, `
			INSERT INTO instance_cluster.node_event (
				node_id, content, payload, target_address,
				target_device, target_login_id
			)
			SELECT id, $1, $2, $3, $4, $5
			FROM instance_cluster.node
			WHERE id            = ANY($6)
			AND   date_check_in > $7
		`, content, payloadJson, address, device, loginId, nodeIds, checkInCutOff); err != nil {
			return err
		}
	}
	return nil
}
func createEventsForOtherNodes(content string, payload interface{}, target types.ClusterEventTarget) error {
	return CreateEventForNodes([]uuid.UUID{}, content, payload, target)
}
