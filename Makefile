MAELSTROM_DIR = maelstrom
ECHO_DIR = challenge-1-echo
UNIQUE_ID_DIR = challenge-2-unique-id

echo:
	go build -C $(ECHO_DIR) -o bin
	./$(MAELSTROM_DIR)/maelstrom test\
		-w echo\
		--bin $(ECHO_DIR)/bin\
		--node-count 1\
		--time-limit 3

unique-id:
	go build -C $(UNIQUE_ID_DIR) -o bin
	./$(MAELSTROM_DIR)/maelstrom test\
		-w unique-ids\
		--bin $(UNIQUE_ID_DIR)/bin\
		--time-limit 30\
		--rate 1000\
		--node-count 3\
		--availability total\
		--nemesis partition
