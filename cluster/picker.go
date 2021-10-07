package cluster

import "github.com/MrEhbr/pgxext/conn"

func defaultConnPicker(db Conn, _ string) conn.Querier {
	return db.Replica()
}
