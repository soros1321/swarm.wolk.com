
.PHONY: all enssimulation dbchunkstore server keymanager bplus client kademliadb hashdb

all:
	@echo "test all."
	@echo "test swarmdb."
	-go test ./swarmdb_test.go
	@echo "test enssimulation."
	-go test ./enssimulation_test.go
	@echo "test dbchunkstore."
	-go test ./dbchunkstore_test.go
	@echo "test keymanager."
	-go test ./keymanager_test.go
	@echo "test bplus."
	-go test ./bplus_test.go
	@echo "test query."
	-go test ./query_test.go
	@echo "test client."
	-go test ./client_test.go
	@echo "test kademliadb."
	-go test ./kademliadb_test.go
	@echo "test hashdb."
	-go test ./hashdb_test.go
	@echo "test netstat."
	-go test ./netstat_test.go

enssimulation:
	@echo "test enssimulation."
	go test ./enssimulation_test.go

dbchunkstore:
	@echo "test dbchunkstore."
	go test ./dbchunkstore_test.go

keymanager:
	@echo "test keymanager."
	go test ./keymanager_test.go

bplus:
	@echo "test bplus."
	go test bplus_test.go

query:
	@echo "test query."
	go test ./query_test.go

swarmdb:
	@echo "test swarmdb."
	go test ./swarmdb_test.go

client:
	@echo "test client."
	go test client_test.go

kademliadb:
	@echo "test kademliadb."
	go test kademliadb_test.go

hashdb:
	@echo "test hashdb."
	go test hashdb_test.go

netstat:
	@echo "test netstat."
	go test ./netstat_test.go
