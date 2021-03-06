// Copyright (c) 2018 Wolk Inc.  All rights reserved.

// The SWARMDB library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The SWARMDB library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package swarmdb

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	//"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"math/big"
	"os"
	"time"
)

var (
	netCounter NetstatFile
)

const (
	minChunkSize = 4000
)

type DBChunk struct {
	Key          []byte // 32
	Val          []byte // 4096
	Owner        []byte // 42
	TableName    []byte // 32
	TableId      []byte // 32
	ChunkBirthDT int64
	ChunkStoreDT int64
}

type ChunkLog struct {
	Farmer           string `json:"farmer"`
	ChunkID          string `json:"chunkID"`
	ChunkHash        []byte `json:"-"`
	ChunkBD          int    `json:"chunkBD"`
	ChunkSD          int    `json:"chunkSD"`
	ReplicationLevel int    `json:"rep"`
	Renewable        int    `json:"renewable"`
	//Claimable        int
}

type ChunkStats struct {
	CurrentTS   int64 `json:"CurrentTS"`
	ChunkRead   int64 `json:"ChunkRead"`
	ChunkWrite  int64 `json:"ChunkWrite"`
	ChunkStored int64 `json:"ChunkStored"`
}

func (self *DBChunkstore) MarshalJSON() (data []byte, err error) {
	logDT := time.Now()
	self.netstat.CStat["ChunkRL"].Add(self.netstat.CStat["ChunkR"], self.netstat.CStat["ChunkRL"])
	self.netstat.CStat["ChunkWL"].Add(self.netstat.CStat["ChunkW"], self.netstat.CStat["ChunkWL"])

	err = self.GetChunkStored()
	if err != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:MarshalJSON] GetChunkStored %s", err.Error())}
	}

	fileInfo, err := os.Stat(self.filepath)
	if err != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:MarshalJSON] Stat %s", err.Error())}
	} else {
		deltaBS := new(big.Int).SetInt64(fileInfo.Size())
		self.netstat.BStat["ByteS"].Sub(deltaBS, self.netstat.BStat["ByteSL"])
		self.netstat.BStat["ByteSL"] = deltaBS
	}

	var file = &NetstatFile{
		NodeID:        self.netstat.NodeID,
		WalletAddress: self.farmer.Hex(),
		Ticket:        make(map[string]string),
		ChunkStat:     make(map[string]string),
		ByteStat:      make(map[string]string),
		LaunchDT:      self.netstat.LaunchDT,
		LReadDT:       self.netstat.LReadDT,
		LWriteDT:      self.netstat.LWriteDT,
		LogDT:         &logDT,
	}

	for cc, cv := range self.netstat.CStat {
		file.ChunkStat[cc] = cv.String()
		if cc == "ChunkR" || cc == "ChunkW" || cc == "ChunkS" {
			self.netstat.CStat[cc] = big.NewInt(0)
		}
	}

	for bc, bv := range self.netstat.BStat {
		file.ByteStat[bc] = bv.String()
		if bc == "ByteR" || bc == "ByteS" || bc == "ByteW" {
			self.netstat.BStat[bc] = big.NewInt(0)
		}
	}

	for ticket, reward := range self.netstat.Claim {
		file.Ticket[ticket] = reward.String()
	}

	data, err = json.Marshal(file)
	if err != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:MarshalJSON] Marshal %s", err.Error())}
	} else {
		return data, nil
	}
}

func (self *DBChunkstore) UnmarshalJSON(data []byte) (err error) {
	var file = NetstatFile{
		Claim: make(map[string]*big.Int),
		CStat: make(map[string]*big.Int),
		BStat: make(map[string]*big.Int),
	}
	err = json.Unmarshal(data, &file)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:UnmarshalJSON]%s", err.Error())}
	}

	self.farmer = common.HexToAddress(file.WalletAddress)

	var ok bool

	for ticket, reward := range file.Ticket {
		file.Claim[ticket], ok = new(big.Int).SetString(reward, 10)
		if !ok {
			return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:UnmarshalJSON] Ticket %v amount set: unable to convert string to big integer: %v", ticket, reward)}
		}
	}

	for cc, cv := range file.ChunkStat {
		if cc == "ChunkR" || cc == "ChunkW" || cc == "ChunkS" {
			file.CStat[cc] = big.NewInt(0)
		} else {
			file.CStat[cc], ok = new(big.Int).SetString(cv, 10)
			if !ok {
				return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:UnmarshalJSON] %v loading failure: unable to convert string to big integer: %v", cc, cv)}
			}
		}
	}

	for bc, bv := range file.ByteStat {
		if bc == "ByteW" || bc == "ByteR" {
			file.BStat[bc] = big.NewInt(0)
		} else {
			file.BStat[bc], ok = new(big.Int).SetString(bv, 10)
			if !ok {
				return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:UnmarshalJSON] %v loading failure: unable to convert string to big integer: %v", bc, bv)}
			}
		}
	}

	self.netstat = &file
	return nil
}

func (self *DBChunkstore) Save() (err error) {
	data, err := json.MarshalIndent(self, "", " ")
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:Save] MarshalIndent %s", err.Error())}
	}
	fmt.Printf("\n%v\n", string(data))
	err = ioutil.WriteFile(self.statpath, data, os.ModePerm)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:Save] WriteFile %s", err.Error())}
	} else {
		return nil
	}
}

func (self *DBChunkstore) Flush() (err error) {
	data, err := json.Marshal(self)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:Flush] Marshal %s", err.Error())}
	}
	netstatlog, err := os.OpenFile("netstat.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:Flush] OpenFile %s", err.Error())}
	}
	defer netstatlog.Close()
	fmt.Fprintf(netstatlog, "%s\n", data)
	return nil
}

func NewDBChunkStore(path string) (self *DBChunkstore, err error) {
	ts := time.Now()
	claim := make(map[string]*big.Int)
	chunkstat := map[string]*big.Int{"ChunkR": big.NewInt(0), "ChunkW": big.NewInt(0), "ChunkS": big.NewInt(0), "ChunkRL": big.NewInt(0), "ChunkWL": big.NewInt(0), "ChunkSL": big.NewInt(0)}
	bytestat := map[string]*big.Int{"ByteR": big.NewInt(0), "ByteW": big.NewInt(0), "ByteS": big.NewInt(0), "ByteRL": big.NewInt(0), "ByteWL": big.NewInt(0), "ByteSL": big.NewInt(0)}

	fmt.Printf("Opening %s\n", path)
	db, err := sql.Open("sqlite3", path)
	if err != nil || db == nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:NewDBChunkStore] Open %s", err.Error())}
	}

	//Local Chunk table
	sql_table := `
    CREATE TABLE IF NOT EXISTS chunk (
    chunkKey TEXT NOT NULL PRIMARY KEY,
    chunkVal BLOB,
    payer TEXT,
    encrypted INTEGER DEFAULT 1,
    renewal INTEGER DEFAULT 1,
    minReplication INTEGER DEFAULT 1,
    maxReplication INTEGER DEFAULT 1,
    version INTEGER DEFAULT 0,
    chunkBirthDT DATETIME,
    chunkStoreDT DATETIME
    );
    `
	netstat_table := `
    CREATE TABLE IF NOT EXISTS netstat (
    statDT  DATETIME NOT NULL PRIMARY KEY,
    rcnt INTEGER DEFAULT 0,
    wcnt INTEGER DEFAULT 0,
    scnt INTEGER DEFAULT 0
    );
    `
	_, err = db.Exec(sql_table)
	//TODO: confirm _ doesn't need handling/checking
	if err != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:NewDBChunkStore] Exec - SQLite Chunk Table Creation %s", err.Error())}
	}
	_, err = db.Exec(netstat_table)
	//TODO: confirm _ doesn't need handling/checking
	if err != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:NewDBChunkStore] Exec - SQLite Stat Table Creation %s", err.Error())}
	}
	config, errConfig := LoadSWARMDBConfig(SWARMDBCONF_FILE)
	if errConfig != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:NewDBChunkStore] LoadSWARMDBConfig - KeyManager Config Loading %s", errConfig.Error())}
	}
	km, errKM := NewKeyManager(&config)
	if errKM != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:NewDBChunkStore] NewKeyManager %s", errKM.Error())}
	}

	userWallet := config.Address
	nodeid := config.GetNodeID()
	walletAddr := common.HexToAddress(userWallet)

	netstat := NetstatFile{
		NodeID:        nodeid,
		WalletAddress: userWallet,
		LaunchDT:      &ts,
		CStat:         chunkstat,
		BStat:         bytestat,
		Claim:         claim,
	}

	self = &DBChunkstore{
		db:       db,
		km:       &km,
		farmer:   walletAddr,
		netstat:  &netstat,
		filepath: path,
		statpath: "netstat.json",
	}
	return self, nil
}

func (self *DBChunkstore) GetKeyManager() (km *KeyManager) {
	return self.km
}

func LoadDBChunkStore(path string) (self *DBChunkstore, err error) {
	var data []byte
	defaultDBPath := "netstat.json"

	data, errLoad := ioutil.ReadFile(defaultDBPath)
	if errLoad != nil {
		self, err = NewDBChunkStore(path)
		if err != nil {
			return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:LoadDBChunkStore] %s", err.Error())}
		} else {
			//TODO: load_err fallback should potentially be marked as warning
			return self, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:LoadDBChunkStore] Load Error %s | Generating new netstatlog in %s", errLoad.Error(), defaultDBPath)}
		}
	}

	self = new(DBChunkstore)
	self.netstat = new(NetstatFile)
	errParse := json.Unmarshal(data, &self)
	if errParse != nil {
		self, err = NewDBChunkStore(path)
		if err != nil {
			return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:LoadDBChunkStore] NewDBChunkStore %s", err.Error())}
		} else {
			//TODO: parse_err fallback should potentially be marked as warning
			return self, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:LoadDBChunkStore] NewDBChunkStore Parsing Error %s | Generating new netstatlog in %s", errParse.Error(), defaultDBPath)}
		}
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil || db == nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:LoadDBChunkStore] sql.Open Error %s", err.Error())}
	}

	config, errConfig := LoadSWARMDBConfig(SWARMDBCONF_FILE)
	if errConfig != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:LoadDBChunkStore] LoadSWARMDBConfig - KeyManager Config Error %s", errConfig.Error())}
	}

	km, errKm := NewKeyManager(&config)
	if errKm != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:LoadDBChunkStore] NewKeyManager Creation Error %s", errKm.Error())}
	}

	self.db = db
	self.km = &km
	self.filepath = path
	self.statpath = defaultDBPath
	return self, nil
}

func (self *DBChunkstore) StoreKChunk(u *SWARMDBUser, key []byte, val []byte, encrypted int) (err error) {
	//TODO get OWNER from CHUNK or get it from swarmdb into dbchunkstore
	ts := time.Now()
	if len(val) < minChunkSize {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:StoreKChunk] Chunk too small (< %s)| %x", minChunkSize, val)}
	}

	sql_add := `INSERT OR REPLACE INTO chunk ( chunkKey, chunkVal, encrypted, chunkBirthDT, chunkStoreDT, renewal, minReplication, maxReplication, payer, version) values(?, ?, ?, COALESCE((SELECT chunkBirthDT FROM chunk WHERE chunkKey=?),CURRENT_TIMESTAMP), COALESCE((SELECT chunkStoreDT FROM chunk WHERE chunkKey=? ), CURRENT_TIMESTAMP), ?, ?, ?, ?, COALESCE((SELECT version+1 FROM chunk where chunkKey=?),0))`
	stmt, err := self.db.Prepare(sql_add)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:StoreKChunk] Prepare %s", err.Error())}
	}
	defer stmt.Close()

	recordData := val[577:4096]
	if encrypted == 1 {
		recordData = self.km.EncryptData(u, recordData)
	}

	var finalSdata [8192]byte
	copy(finalSdata[0:577], val[0:577])
	copy(finalSdata[577:], recordData)
	_, err2 := stmt.Exec(key[:32], finalSdata[0:], encrypted, key[:32], key[:32], u.AutoRenew, u.MinReplication, u.MaxReplication, u.Address, key[:32])
	if err2 != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:StoreKChunk] Exec - Insert%s | data:%x | Encrypted: %s ", err2.Error(), finalSdata, encrypted)}
	}
	stmt.Close()
	self.netstat.LWriteDT = &ts
	self.netstat.CStat["ChunkW"].Add(self.netstat.CStat["ChunkW"], big.NewInt(1))
	return nil
}

func (self *DBChunkstore) RetrieveKChunk(u *SWARMDBUser, key []byte) (val []byte, err error) {
	ts := time.Now()
	val = make([]byte, 8192)
	sql := `SELECT chunkKey, chunkVal, chunkBirthDT, chunkStoreDT, encrypted FROM chunk WHERE chunkKey = $1`
	stmt, err := self.db.Prepare(sql)
	if err != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:RetrieveKChunk] Prepare %s | %s", sql, err.Error())}
	}
	defer stmt.Close()

	rows, err := stmt.Query(key)
	if err != nil {
		//fmt.Printf("Error preparing sql [%s] Err: [%s]", sql, err)
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:RetrieveKChunk] Query %s | %s", sql, err.Error())}
	}
	defer rows.Close()

	for rows.Next() {
		var kV []byte
		var bdt []byte
		var sdt []byte
		var enc int

		err2 := rows.Scan(&kV, &val, &bdt, &sdt, &enc)
		if err2 != nil {
			return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:RetrieveKChunk] Scan %s", err2.Error())}
		}
		//TODO: (Rodney) parse encrypted chunk
		jsonRecord := val[577:]
		trimmedJson := bytes.TrimRight(jsonRecord, "\x00")
		var retVal []byte
		retVal = trimmedJson
		if enc == 1 {
			retVal, err = self.km.DecryptData(u, trimmedJson)
			if err != nil {
				return val, err
			}

			retVal = bytes.TrimRight(retVal, "\x00")
		}
		self.netstat.LReadDT = &ts
		self.netstat.CStat["ChunkR"].Add(self.netstat.CStat["ChunkR"], big.NewInt(1))
		return retVal, nil
	}
	return val, nil
}

func (self *DBChunkstore) StoreChunk(u *SWARMDBUser, val []byte, encrypted int) (key []byte, err error) {
	ts := time.Now()
	if len(val) < minChunkSize {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:StoreChunk] Chunk too small (< %s)| %x", minChunkSize, val)}
	}
	inp := make([]byte, minChunkSize)
	copy(inp, val[0:minChunkSize])
	h := sha256.New()
	h.Write([]byte(inp))
	key = h.Sum(nil)

	sql_add := `INSERT OR REPLACE INTO chunk ( chunkKey, chunkVal, encrypted, chunkBirthDT, chunkStoreDT, renewal, minReplication, maxReplication, payer, version) values(?, ?, ?, COALESCE((SELECT chunkStoreDT FROM chunk WHERE chunkKey=?),CURRENT_TIMESTAMP), COALESCE((SELECT chunkStoreDT FROM chunk WHERE chunkKey=? ), CURRENT_TIMESTAMP), ?, ?, ?, ?, COALESCE((SELECT version+1 FROM chunk where chunkKey=?),0))`
	stmt, err := self.db.Prepare(sql_add)
	if err != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:StoreChunk] Prepare %s", err.Error())}
	}
	defer stmt.Close()

	var chunkVal []byte
	chunkVal = val
	if encrypted == 1 {
		chunkVal = self.km.EncryptData(u, val)
	}
	_, err2 := stmt.Exec(key, chunkVal, encrypted, key, key, u.AutoRenew, u.MinReplication, u.MaxReplication, u.Address, key)
	//TODO: confirm _ doesn't need handling/checking
	if err2 != nil {
		fmt.Printf("\nError Inserting into Table: [%s]", err2.Error())
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:StoreChunk] Exec %s | data:%s | encrypted:%s", err2.Error(), chunkVal, encrypted)}
	}
	stmt.Close()
	self.netstat.LWriteDT = &ts
	self.netstat.CStat["ChunkW"].Add(self.netstat.CStat["ChunkW"], big.NewInt(1))
	//self.netstat.CStat["ChunkS"].Add(self.netstat.CStat["ChunkS"], big.NewInt(1))
	return key, nil
}

func (self *DBChunkstore) RetrieveChunk(u *SWARMDBUser, key []byte) (val []byte, err error) {
	ts := time.Now()
	val = make([]byte, 8192)
	sql := `SELECT chunkVal, encrypted FROM chunk WHERE chunkKey = $1`
	stmt, err := self.db.Prepare(sql)
	if err != nil {
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:RetrieveChunk] Prepare %s | %s", sql, err.Error())}
	}
	defer stmt.Close()

	rows, err := stmt.Query(key)
	if err != nil {
		//fmt.Printf("Error preparing sql [%s] Err: [%s]", sql, err)
		return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:RetrieveChunk] Query %s | %s", sql, err.Error())}
	}
	defer rows.Close()

	for rows.Next() {
		var enc int
		err2 := rows.Scan(&val, &enc)
		if err2 != nil {
			return nil, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:RetrieveChunk] Scan %s", err2.Error())}
		}
		var retVal []byte
		retVal = val
		if enc == 1 {
			retVal, err = self.km.DecryptData(u, val)
			if err != nil {
				return retVal, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:RetrieveChunk] DecryptData %s", err2.Error())}
			}
		}
		self.netstat.LReadDT = &ts
		self.netstat.CStat["ChunkR"].Add(self.netstat.CStat["ChunkR"], big.NewInt(1))
		return retVal, nil
	}
	return val, nil
}

func valid_type(typ string) (valid bool) {
	if typ == "X" || typ == "D" || typ == "H" || typ == "K" || typ == "C" {
		return true
	}
	return false
}

func (self *DBChunkstore) PrintDBChunk(columnType ColumnType, hashid []byte, c []byte) {
	nodetype := string(c[4096-65 : 4096-64])
	if valid_type(nodetype) {
		fmt.Printf("Chunk %x ", hashid)
		fmt.Printf(" NodeType: %s ", nodetype)
		childtype := string(c[4096-66 : 4096-65])
		if valid_type(childtype) {
			fmt.Printf(" ChildType: %s ", childtype)
		}
		fmt.Printf("\n")
		if nodetype == "D" {
			p := make([]byte, 32)
			n := make([]byte, 32)
			copy(p, c[4096-64:4096-32])
			copy(n, c[4096-64:4096-32])
			if IsHash(p) {
				fmt.Printf(" PREV: %x ", p)
			} else {
				fmt.Printf(" PREV: *NULL* ", p)
			}
			if IsHash(n) {
				fmt.Printf("\tNEXT: %x ", n)
			} else {
				fmt.Printf("\tNEXT: *NULL* ", p)
			}
			fmt.Printf("\n")

		}
	}

	k := make([]byte, 32)
	v := make([]byte, 32)
	for i := 0; i < 32; i++ {
		copy(k, c[i*64:i*64+32])
		copy(v, c[i*64+32:i*64+64])
		if EmptyBytes(k) && EmptyBytes(v) {
		} else {
			fmt.Printf(" %d:\t%s\t%s\n", i, KeyToString(columnType, k), ValueToString(v))
		}
	}
	fmt.Printf("\n")
}

func (self *DBChunkstore) ScanAll() (err error) {
	ts := time.Now()
	sql_readall := `SELECT chunkKey, chunkVal,strftime('%s',chunkStoreDT) FROM chunk ORDER BY chunkStoreDT DESC`
	rows, err := self.db.Query(sql_readall)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:ScanAll] Query %s", err.Error())}
	}
	defer rows.Close()

	var rcnt int64
	var result []DBChunk
	for rows.Next() {
		c := DBChunk{}
		err2 := rows.Scan(&c.Key, &c.Val, &c.ChunkStoreDT)
		if err2 != nil {
			return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:ScanAll] Scan %s", err2.Error())}
		}
		rcnt++
		result = append(result, c)
	}
	rows.Close()

	sql_chunkRead := `INSERT OR REPLACE INTO netstat (statDT, rcnt) values(CURRENT_TIMESTAMP, ?)`
	stmt, err := self.db.Prepare(sql_chunkRead)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:ScanAll] Prepare %s", err.Error())}
	}
	defer stmt.Close()

	_, err2 := stmt.Exec(rcnt)
	// TODO: confirm _ doesn't need handling/checking
	if err2 != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:ScanAll] Exec Error updating stat Table: %s", err2.Error())}
	}
	stmt.Close()
	self.netstat.LReadDT = &ts
	self.netstat.CStat["ChunkR"].Add(self.netstat.CStat["ChunkR"], new(big.Int).SetInt64(rcnt))
	return nil
}

func (self *DBChunkstore) GenerateFarmerLog() (err error) {

	farmerAddr := self.farmer.Hex()

	/*
	   currentTS:= time.Now().Unix()
	   contractInterval := 3600*7 //Test renewal interval
	*/

	sql_readall := `SELECT chunkKey,strftime('%s',chunkBirthDT) as chunkBirthTS, strftime('%s',chunkStoreDT) as chunkStoreTS, maxReplication, renewal FROM chunk where maxReplication > 0 ORDER BY chunkStoreTS DESC`
	rows, err := self.db.Query(sql_readall)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:GenerateFarmerLog] Query %s", err.Error())}
	}
	defer rows.Close()

	var result []ChunkLog
	for rows.Next() {
		c := ChunkLog{}
		c.Farmer = farmerAddr

		err2 := rows.Scan(&c.ChunkHash, &c.ChunkBD, &c.ChunkSD, &c.ReplicationLevel, &c.Renewable)
		if err2 != nil {
			return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:GenerateFarmerLog] Scan %s", err2.Error())}
		}
		c.ChunkID = fmt.Sprintf("%x", c.ChunkHash)
		chunklog, err := json.Marshal(c)
		if err != nil {
			return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:GenerateFarmerLog] Marshal %s", err2.Error())}
		}
		fmt.Printf("%s\n", chunklog)
		result = append(result, c)
	}
	rows.Close()
	return nil
}

//TODO: Finish implementation/document -- Seems there's no error case. Are we not yet checking appropriately?  Or is it not needed?
func (self *DBChunkstore) ClaimAll() (err error) {
	fmt.Printf("netCounter: %v\n", netCounter)
	fmt.Printf("self: %v\n", self)

	ticket := "9f2018c7dc1e31fb6708fd6bd0f8975bf704e5a0e8465fbef2b5e7e5fc37c4d8"
	reward := 121
	self.netstat.Claim[ticket] = new(big.Int).SetInt64(int64(reward))
	return nil
}

func (self *DBChunkstore) GetChunkStored() (err error) {
	sql_chunkTally := `SELECT count(*) FROM chunk`
	rows, err := self.db.Query(sql_chunkTally)
	if err != nil {
		return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:GetChunkStored] Query %s", err.Error())}
	}
	defer rows.Close()

	var result []ChunkStats
	chunkStored := int64(0)
	for rows.Next() {
		c := ChunkStats{}
		err2 := rows.Scan(&c.ChunkStored)
		if err2 != nil {
			return &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:GetChunkStored] Scan %s", err2.Error())}
		}
		chunkStored += c.ChunkStored
		result = append(result, c)
	}
	rows.Close()
	self.netstat.CStat["ChunkS"].Sub(new(big.Int).SetInt64(chunkStored), self.netstat.CStat["ChunkSL"])
	self.netstat.CStat["ChunkSL"] = new(big.Int).SetInt64(chunkStored)
	return nil
}

func (self *DBChunkstore) GetChunkStat() (res string, err error) {
	sql_chunkTally := `SELECT strftime('%s',statDT) as STS, sum(rcnt), sum(wcnt), sum(scnt) FROM netstat group by strftime('%s',statDT) order by STS DESC`
	rows, err := self.db.Query(sql_chunkTally)
	if err != nil {
		return res, &SWARMDBError{message: fmt.Sprintf("[Query]%s", err.Error())}
	}
	defer rows.Close()

	var result []ChunkStats
	for rows.Next() {
		c := ChunkStats{}
		err2 := rows.Scan(&c.CurrentTS, &c.ChunkRead, &c.ChunkWrite, &c.ChunkStored)
		if err2 != nil {
			return res, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:GetChunkStat] Scan %s", err.Error())}
		}
		fmt.Printf("[stat] Time %v => Read:%v | Write:%v | Stored:%v\n", c.CurrentTS, c.ChunkRead, c.ChunkWrite, c.ChunkStored)
		result = append(result, c)
	}
	rows.Close()

	output, err := json.Marshal(result)
	if err != nil {
		return res, &SWARMDBError{message: fmt.Sprintf("[dbchunkstore:GetChunkStat] Marshal %s", err.Error())}
	} else {
		return string(output), nil
	}
}
