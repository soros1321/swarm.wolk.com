package storage

import (
	"crypto/sha256"
	"database/sql"
	//"encoding/binary"
	"fmt"
	"github.com/ethereum/go-ethereum/swarmdb/common"
	"github.com/ethereum/go-ethereum/swarmdb/keymanager"
	_ "github.com/mattn/go-sqlite3"
	//"math"
	"time"
)

type DBChunkstore struct {
	filepath string
	db       *sql.DB
	km       *keymanager.KeyManager
}

type DBChunk struct {
	Key         []byte // 32
	Val         []byte // 4096
	Owner       []byte // 42
	BuyAt       []byte // 32
	Blocknumber []byte // 32
	Tablename   []byte // 32
	TableId     []byte // 32
	StoreDT     *time.Time
}

func NewDBChunkStore(path string) (dbcs DBChunkstore, err error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return dbcs, err
	}
	if db == nil {
		return dbcs, err
	}
	dbcs.db = db
	dbcs.filepath = path
	// create table if not exists
	sql_table := `
	CREATE TABLE IF NOT EXISTS chunk (
	chunkKey TEXT NOT NULL PRIMARY KEY,
	chunkVal BLOB,
	Owner TEXT,
	BuyAt TEXT,
	BlockNumber TEXT,
	Tablename TEXT,
	Tableid TEXT,
	storeDT DATETIME
	);
	`
	_, err = db.Exec(sql_table)
	if err != nil {
		fmt.Printf("Error Creating Table")
		return dbcs, err
	}
	km, errKm := keymanager.NewKeyManager("/tmp/blah")
	if errKm != nil {
		fmt.Printf("Error Creating KeyManager")
		return dbcs, err
	}
	dbcs.km = &km
	return dbcs, nil
}

func (self *DBChunkstore) StoreKChunk(k []byte, v []byte) (err error) {
	if len(v) < minChunkSize {
		return fmt.Errorf("chunk too small") // should be improved
	}

	sql_add := `INSERT OR REPLACE INTO chunk ( chunkKey, chunkVal, storeDT ) values(?, ?, CURRENT_TIMESTAMP)`
	stmt, err := self.db.Prepare(sql_add)
	if err != nil {
		fmt.Printf("\nError Preparing into Table: [%s]", err)
		return (err)
	}
	defer stmt.Close()

	encVal := self.km.EncryptData(v)
	_, err2 := stmt.Exec(k, encVal)
	fmt.Printf("\noriginal val [%v] encoded to [%v]", v, encVal)
	if err2 != nil {
		fmt.Printf("\nError Inserting into Table: [%s]", err)
		return (err2)
	}
	return nil
}

const (
	minChunkSize = 4000
)

func (self *DBChunkstore) StoreChunk(v []byte) (k []byte, err error) {
	if len(v) < minChunkSize {
		return k, fmt.Errorf("chunk too small") // should be improved
	}
	inp := make([]byte, minChunkSize)
	copy(inp, v[0:minChunkSize])
	h := sha256.New()
	h.Write([]byte(inp))
	k = h.Sum(nil)

	sql_add := `INSERT OR REPLACE INTO chunk ( chunkKey, chunkVal, storeDT ) values(?, ?, CURRENT_TIMESTAMP)`
	stmt, err := self.db.Prepare(sql_add)
	if err != nil {
		return k, err
	}
	defer stmt.Close()

	encVal := self.km.EncryptData(v)
	_, err2 := stmt.Exec(k, encVal)
	if err2 != nil {
		fmt.Printf("\nError Inserting into Table: [%s]", err)
		return k, err2
	}

	return k, nil
}

func (self *DBChunkstore) RetrieveChunk(key []byte) (val []byte, err error) {
	val = make([]byte, 4096)
	sql := `SELECT chunkVal FROM chunk WHERE chunkKey = $1`
	stmt, err := self.db.Prepare(sql)
	if err != nil {
		fmt.Printf("Error preparing sql [%s] Err: [%s]", sql, err)
		return val, err
	}
	defer stmt.Close()

	//rows, err := stmt.Query()
	rows, err := stmt.Query(key)
	if err != nil {
		fmt.Printf("Error preparing sql [%s] Err: [%s]", sql, err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err2 := rows.Scan(&val)
		fmt.Printf("RETVALUE is [%s] Err: ", val)
		if err2 != nil {
			return nil, err2
		}
		decVal := self.km.DecryptData(val)
		fmt.Printf("DECVALUE is [%s] Err: ", val)
		return decVal, nil
	}
	return val, nil
}

func valid_type(typ string) (valid bool) {
	if typ == "X" || typ == "D" || typ == "H" || typ == "K" || typ == "C" {
		return true
	}
	return false
}

func (self *DBChunkstore) PrintDBChunk(keytype common.KeyType, hashid []byte, c []byte) {
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
			if common.IsHash(p) {
				fmt.Printf(" PREV: %x ", p)
			} else {
				fmt.Printf(" PREV: *NULL* ", p)
			}
			if common.IsHash(n) {
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
		if common.EmptyBytes(k) && common.EmptyBytes(v) {
		} else {
			fmt.Printf(" %d:\t%s\t%s\n", i, common.KeyToString(keytype, k), common.ValueToString(v))
		}
	}
	fmt.Printf("\n")
}

func (self *DBChunkstore) ScanAll() (err error) {
	sql_readall := `SELECT chunkKey, chunkVal, storeDT FROM chunk ORDER BY datetime(storeDT) DESC`
	rows, err := self.db.Query(sql_readall)
	if err != nil {
		return err
	}
	defer rows.Close()

	var result []DBChunk
	for rows.Next() {
		c := DBChunk{}
		err2 := rows.Scan(&c.Key, &c.Val, &c.StoreDT)
		if err2 != nil {
			return err2
		}
		result = append(result, c)
	}
	return nil
}
