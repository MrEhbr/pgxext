package cluster

import "github.com/MrEhbr/pgxext/v2/conn"

func defaultConnPicker(db Conn, _ string) conn.Querier {
	return db.Replica()
}
