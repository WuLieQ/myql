// Go MySQL Driver - A MySQL-Driver for Go's database/sql package
//
// Copyright 2012 The Go-MySQL-Driver Authors. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package mysql

import (
	"database/sql/driver"
)

type mysqlStmt struct {
	mc         *mysqlConn
	id         uint32
	paramCount int
	columns    []mysqlField // cached from the first query
}

func (stmt *mysqlStmt) Close() error {
	if stmt.mc == nil || stmt.mc.netConn == nil {
		return errInvalidConn
	}

	err := stmt.mc.writeCommandPacketUint32(comStmtClose, stmt.id)

	if stmt.columns != nil {
		putFields(stmt.columns)
		stmt.columns = nil
	}
	stmt.mc = nil

	return err
}

func (stmt *mysqlStmt) NumInput() int {
	return stmt.paramCount
}

func (stmt *mysqlStmt) Exec(args []driver.Value) (driver.Result, error) {
	// Send command
	err := stmt.writeExecutePacket(args)
	if err != nil {
		return nil, err
	}

	mc := stmt.mc

	mc.affectedRows = 0
	mc.insertId = 0

	// Read Result
	resLen, err := mc.readResultSetHeaderPacket()
	if err == nil {
		if resLen > 0 {
			// Columns
			err = mc.readUntilEOF()
			if err != nil {
				return nil, err
			}

			// Rows
			err = mc.readUntilEOF()
		}
		if err == nil {
			return &mysqlResult{
				affectedRows: int64(mc.affectedRows),
				insertId:     int64(mc.insertId),
			}, nil
		}
	}

	return nil, err
}

func (stmt *mysqlStmt) Query(args []driver.Value) (driver.Rows, error) {
	// Send command
	err := stmt.writeExecutePacket(args)
	if err != nil {
		return nil, err
	}

	mc := stmt.mc

	// Read Result
	resLen, err := mc.readResultSetHeaderPacket()
	if err != nil {
		return nil, err
	}

	rows := newMysqlRows()
	rows.mc = stmt.mc
	rows.binary = true

	if resLen > 0 {
		// Columns
		// If not cached, read them and cache them
		if stmt.columns == nil {
			rows.columns, err = mc.readColumns(resLen)
			stmt.columns = rows.columns
		} else {
			rows.columns = stmt.columns
			err = mc.readUntilEOF()
		}
	}

	return &mysqlRowsI{rows}, err
}
