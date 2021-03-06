/*
 * Copyright(c)  2019 Lianjia, Inc.  All Rights Reserved
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *     http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rebuild

import (
	"fmt"
	"strings"

	"github.com/LianjiaTech/lightning/common"

	"github.com/siddontang/go-mysql/replication"
	lua "github.com/yuin/gopher-lua"
)

// DeleteRebuild ...
func DeleteRebuild(event *replication.BinlogEvent) string {
	switch common.Config.Rebuild.Plugin {
	case "sql":
		DeleteQuery(event)
	case "flashback":
		DeleteRollbackQuery(event)
	case "stat":
		DeleteStat(event)
	case "lua":
		DeleteLua(event)
	default:
	}
	return ""
}

// DeleteQuery build original delete SQL
func DeleteQuery(event *replication.BinlogEvent) {
	table := RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	deleteQuery(table, values)
}

func deleteQuery(table string, values [][]string) {
	if ok := PrimaryKeys[table]; ok != nil {
		for _, value := range values {
			var where []string
			for _, col := range PrimaryKeys[table] {
				for i, c := range Columns[table] {
					if c == col {
						if value[i] == "NULL" {
							where = append(where, fmt.Sprintf("%s IS NULL", col))
						} else {
							where = append(where, fmt.Sprintf("%s = %s", col, value[i]))
						}
					}
				}
			}
			fmt.Printf("DELETE FROM %s WHERE %s LIMIT 1;\n", table, strings.Join(where, " AND "))
		}
	} else {
		for _, value := range values {
			var where []string
			for i, v := range value {
				col := fmt.Sprintf("@%d", i)
				if v == "NULL" {
					where = append(where, fmt.Sprintf("%s IS NULL", col))
				} else {
					where = append(where, fmt.Sprintf("%s = %s", col, v))
				}
			}
			fmt.Printf("-- DELETE FROM %s WHERE %s LIMIT 1;\n", table, strings.Join(where, " AND "))
		}
	}
}

// DeleteRollbackQuery build rollback insert SQL
func DeleteRollbackQuery(event *replication.BinlogEvent) {
	table := RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	insertQuery(table, values)
}

// DeleteStat ...
func DeleteStat(event *replication.BinlogEvent) {
	table := RowEventTable(event)
	if TableStats[table] != nil {
		TableStats[table]["delete"]++
	} else {
		TableStats[table] = map[string]int64{"delete": 1}
	}
}

// DeleteLua ...
func DeleteLua(event *replication.BinlogEvent) {
	if common.Config.Rebuild.LuaScript == "" || event == nil {
		return
	}

	table := RowEventTable(event)
	ev := event.Event.(*replication.RowsEvent)
	values := BuildValues(ev)

	// lua function
	f := lua.P{
		Fn:      Lua.GetGlobal("DeleteRewrite"),
		NRet:    0,
		Protect: true,
	}
	// lua value
	v := lua.LString(table)
	for _, value := range values {
		LuaStringList("GoValues", value)
		if err := Lua.CallByParam(f, v); err != nil {
			common.Log.Error(err.Error())
			return
		}
	}
}
