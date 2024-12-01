package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	db "dumbo_ms/database"
	leveldb "dumbo_ms/database/leveldb"
	"dumbo_ms/log"
	pb "dumbo_ms/structure"

	"gopkg.in/yaml.v3"
)

type DBtest struct {
	DBPath  string `yaml:"DBPath"`
	LogFile string `yaml:"LogFile"`
	HbDB    db.MsDB
	HbLog   log.MsLog
	Ldb     leveldb.DB
}

func main() {

	var idf = flag.Int("id", 0, "-id")
	flag.Parse()
	id := *idf

	gopath := os.Getenv("GOPATH")
	readBytes, err := os.ReadFile(gopath + "/src/dumbo_ms/config/configure.yaml")
	if err != nil {
		panic(err)
	}

	var hbTest DBtest
	err = yaml.Unmarshal(readBytes, &hbTest)
	if err != nil {
		panic(err)
	}

	logFileName := fmt.Sprintf("/ms%d.log", id)
	hbTest.HbLog.Init(gopath + hbTest.LogFile + logFileName)

	hbTest.TestDataBase(1)

}

func (DBT *DBtest) TestLevelDB(id int) {
	gopath := os.Getenv("GOPATH")
	path := gopath + DBT.DBPath + fmt.Sprintf("/%d", id)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}
	ldb := *leveldb.CreateDB(path)
	err = ldb.Open()
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second)
	for i := 1; i < 5; i++ {
		ldb.Put([]byte(strconv.Itoa(i)), []byte("123"))
	}

	for i := 1; i < 5; i++ {
		content, err := ldb.Get([]byte(strconv.Itoa(i)))
		if err != nil {
			panic(err)
		}
		fmt.Println(content)
	}
}

func (DBT *DBtest) TestDataBase(id int) {
	hbDBInputCH := make(chan pb.BlockInfo, 100)
	gopath := os.Getenv("GOPATH")
	go DBT.HbDB.Init(gopath+DBT.DBPath+fmt.Sprintf("/%d", id), DBT.HbLog, hbDBInputCH)

	time.Sleep(time.Second)
	for i := 1; i < 5; i++ {
		hbDBInputCH <- pb.BlockInfo{Priority: i, Content: []byte("123")}
	}

	for i := 1; i < 5; i++ {
		content := DBT.HbDB.FindWithPriority(i)
		fmt.Println(content)
	}
}
