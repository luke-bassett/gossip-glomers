MAELSTROM_DIR = maelstrom
CH1_DIR = challenge-1-echo
CH2_DIR = challenge-2-unique-id
CH3A_DIR = challenge-3a-broadcast
CH3B_DIR = challenge-3b-broadcast
CH3C_DIR = challenge-3c-broadcast

ch1:
	go build -C $(CH1_DIR) -o bin
	./$(MAELSTROM_DIR)/maelstrom test\
		-w echo\
		--bin $(CH1_DIR)/bin\
		--node-count 1\
		--time-limit 3

ch2:
	go build -C $(CH2_DIR) -o bin
	./$(MAELSTROM_DIR)/maelstrom test\
		-w unique-ids\
		--bin $(CH2_DIR)/bin\
		--time-limit 30\
		--rate 1000\
		--node-count 3\
		--availability total\
		--nemesis partition

ch3a:
	go build -C $(CH3A_DIR) -o bin
	./$(MAELSTROM_DIR)/maelstrom test\
		-w broadcast\
		--bin $(CH3A_DIR)/bin\
		--node-count 1\
		--time-limit 10\
		--rate 10

ch3b:
	go build -C $(CH3B_DIR) -o bin
	./$(MAELSTROM_DIR)/maelstrom test\
		-w broadcast\
		--bin $(CH3B_DIR)/bin\
		--node-count 5\
		--time-limit 10\
		--rate 10

ch3c:
	go build -C $(CH3C_DIR) -o bin
	./$(MAELSTROM_DIR)/maelstrom test\
		-w broadcast\
		--bin $(CH3C_DIR)/bin\
		--node-count 5\
		--time-limit 10\
		--rate 10\
		--nemesis partition
