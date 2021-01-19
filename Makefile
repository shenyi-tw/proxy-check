
all: run


test1:
	DatabaseHost=127.0.0.1 \
	DatabasePort=5432 \
	DatabaseName=postgres \
	DatabaseUser=postgres \
	DatabasePassword=pass \
	ThreadNum=20 \
	go run .
