package database

import (
	leveldb "dumbo_ms/database/leveldb"
	log "dumbo_ms/log"
	pb "dumbo_ms/structure"
	"os"
	"strconv"
)

type MsDB struct {
	db      *leveldb.DB
	InputCH chan pb.BlockInfo
	Log     log.MsLog
}

func (db *MsDB) Init(path string, dblog log.MsLog, inputCH chan pb.BlockInfo) {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}
	db.Log = dblog
	ldb := *leveldb.CreateDB(path)
	db.db = &ldb
	err = ldb.Open()
	if err != nil {
		panic(err)
	}
	db.InputCH = inputCH
	db.HandleInput()
}

func (db *MsDB) HandleInput() {
	for {
		dbInfo := <-db.InputCH
		if err := db.db.Put([]byte(strconv.Itoa(dbInfo.Priority)), dbInfo.Content); err != nil {
			db.Log.Warn("Database error: " + err.Error())
		}
	}
}

func (db *MsDB) FindWithPriority(key int) []byte {
	msg, err := db.db.Get([]byte(strconv.Itoa(key)))
	if err != nil {
		db.Log.Error("Database error: " + err.Error())
	}
	return msg
}
