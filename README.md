# Coding-Challenge

My solutions to John Crickett's [Coding Challenges](https://codingchallenges.fyi/).
Each folder is a self-contained project with its own README.

| # | Project | Language | Description |
|---|---------|----------|-------------|
| 1 | [01-wc](01-wc/) | Go | Counts lines, words, characters and bytes, matching GNU wc output. |
| 2 | [02-json-parser](02-json-parser/) | Go | Hand-written lexer and recursive-descent parser that validates JSON. |
| 3 | [03-compression](03-compression/) | Go | Lossless file compressor and decompressor using Huffman coding. |
| 4 | [04-cut](04-cut/) | Go | Cuts fields, bytes or characters out of each line of input. |
| 5 | [05-load-balancer](05-load-balancer/) | Go | HTTP load balancer with round-robin scheduling and health checks. |
| 6 | [06-sort](06-sort/) | Rust | sort clone with several algorithms, unique/random modes and external merge sort. |
| 7 | [07-calculator](07-calculator/) | Python | Expression calculator using the shunting-yard algorithm. |
| 8 | [08-redis-server](08-redis-server/) | Go | Redis-compatible server speaking RESP, with expiry and persistence. |
| 9 | [09-grep](09-grep/) | Go | Searches files for regex matches, with recursion, inversion and case options. |
| 10 | [10-uniq](10-uniq/) | Go | Filters or counts adjacent duplicate lines. |
| 11 | [11-web-server](11-web-server/) | Go | Concurrent HTTP server that serves static files. |
| 12 | [12-url-shortener](12-url-shortener/) | Go | Web service that turns long URLs into short codes and redirects them. |
| 13 | [13-diff-tool](13-diff-tool/) | Go | Line-by-line file diff built on the longest common subsequence. |
| 14 | [14-build-your-own-shell](14-build-your-own-shell/) | Go | Interactive shell with built-ins, pipes and command history. |
| 15 | [15-build-your-own-cat](15-build-your-own-cat/) | Go | Concatenates files to stdout, with line numbering options. |
| 16 | [16-irc-client](16-irc-client/) | Go | Terminal IRC client that joins channels and chats. |
| 17 | [17-memcached-server](17-memcached-server/) | Go | Memcached-compatible server supporting the text protocol. |
| 18 | [18-spotify-client](18-spotify-client/) | Python | Command-line Spotify client for searching and controlling playback. |
| 19 | [19-discord-bot](19-discord-bot/) | TypeScript | Discord bot with quotes, challenge lookups and slash commands. |
| 20 | [20-linkedin-carousel](20-linkedin-carousel/) | Python | Turns a Markdown post into a LinkedIn carousel PDF. |
| 21 | [21-sed](21-sed/) | Go | Stream editor supporting substitution, line selection and in-place edits. |
| 22 | [22-dns-resolver](22-dns-resolver/) | Go | Resolves domain names by querying root, TLD and authoritative servers. |
| 23 | [23-traceroute](23-traceroute/) | Go | Traces the network hops to a host using TTL-limited probes. |
| 24 | [24-realtime-chat](24-realtime-chat/) | Go | Chat server and client with rooms over TCP and WebSockets. |
| 25 | [25-build-your-own-nats](25-build-your-own-nats/) | Go | Message broker implementing the NATS publish/subscribe protocol. |
| 26 | [26-build-your-own-git](26-build-your-own-git/) | Go | Git-compatible client: init, add, commit, log and push over smart HTTP. |
| 27 | [27-rate-limiter](27-rate-limiter/) | Go | Token bucket, fixed and sliding window rate limiters behind an HTTP API. |
| 28 | [28-ntp-client](28-ntp-client/) | Go | Queries NTP servers and reports clock offset and delay. |
| 29 | [29-scheduling-automation](29-scheduling-automation/) | Python | Calendly-style booking app that finds free slots across calendars. |
| 30 | [30-lisp-interpreter](30-lisp-interpreter/) | Python | Scheme-flavoured Lisp interpreter with closures and a REPL. |
| 31 | [31-qr-code-generator](31-qr-code-generator/) | Python | Encodes text into QR codes with error correction and masking. |
| 32 | [32-crontab-tool](32-crontab-tool/) | Go | Parses cron expressions, explains them and runs scheduled jobs. |
| 33 | [33-head](33-head/) | Go | Prints the first lines or bytes of files. |
| 34 | [34-jq](34-jq/) | Go | Command-line JSON processor implementing a large subset of jq filters. |
| 35 | [35-google-keep](35-google-keep/) | TypeScript | Sticky-note app with a REST API, logins, labels and search. |
| 36 | [36-pong](36-pong/) | TypeScript | Browser Pong with a computer opponent. |
| 37 | [37-redis-cli-tool](37-redis-cli-tool/) | Go | Interactive Redis command-line client speaking RESP. |
| 38 | [38-network-modelling](38-network-modelling/) | Python | Offline command-line tool for modelling IP networks and routing. |
| 39 | [39-social-media](39-social-media/) | Go | Schedules and publishes posts to social networks through a work queue. |
| 40 | [40-curl](40-curl/) | Go | HTTP client for GET, POST, PUT and DELETE with headers and verbose mode. |
| 41 | [41-load-tester](41-load-tester/) | Go | Fires concurrent HTTP requests and reports latency percentiles. |
| 42 | [42-tr](42-tr/) | Go | Translates, squeezes and deletes characters, with character classes. |
| 43 | [43-tetris](43-tetris/) | TypeScript | Browser Tetris with scoring, levels and piece preview. |
| 44 | [44-dns-forwarder](44-dns-forwarder/) | Go | Forwards DNS queries upstream and caches the answers. |
| 45 | [45-port-scanner](45-port-scanner/) | Go | Concurrent TCP port scanner with host and port ranges. |
| 46 | [46-yq](46-yq/) | Go | YAML processor with jq-style query expressions. |
| 47 | [47-chrome-new-tab-extension](47-chrome-new-tab-extension/) | TypeScript | Chrome extension that replaces the new tab page with a dashboard. |
| 48 | [48-data-privacy](48-data-privacy/) | Go | Service that tokenises sensitive data and stores it encrypted. |
| 49 | [49-password-cracker](49-password-cracker/) | Go | Cracks hashes by brute force, wordlists and rainbow tables. |
| 50 | [50-xargs](50-xargs/) | Go | Builds and runs commands from standard input, optionally in parallel. |
| 51 | [51-http-forward-proxy](51-http-forward-proxy/) | Go | Forward proxy with HTTPS tunnelling and a host blocklist. |
| 52 | [52-docker](52-docker/) | Go | Tiny docker run: pulls a Docker Hub image and runs a command inside it. |
| 53 | [53-bloom-filter](53-bloom-filter/) | Go | Spell checker backed by a Bloom filter saved to disk. |
| 54 | [54-tar](54-tar/) | Go | Creates, lists and extracts tar archives. |
| 55 | [55-xxd](55-xxd/) | Go | Makes hex dumps of files and reverses them back to binary. |
| 56 | [56-chess](56-chess/) | Go | Chess engine with full move rules and a minimax computer player. |
| 57 | [57-snake](57-snake/) | Python | Classic Snake game. |
| 58 | [58-password-manager](58-password-manager/) | Go | Encrypted command-line password vault with a master password. |
| 59 | [59-netcat](59-netcat/) | Go | Reads and writes data across TCP and UDP connections. |
| 60 | [60-pastebin](60-pastebin/) | Go | Web app for sharing text snippets with expiry. |
| 61 | [61-dropbox](61-dropbox/) | Go | Syncs files between clients through a central server. |
| 62 | [62-git-contributions](62-git-contributions/) | Go | Draws a GitHub-style contribution heatmap from git history. |
| 63 | [63-space-invaders](63-space-invaders/) | TypeScript | Browser Space Invaders with waves, bunkers and scoring. |
| 64 | [64-spotify-playlist](64-spotify-playlist/) | Python | Backs up and restores Spotify playlists. |
| 65 | [65-minesweeper](65-minesweeper/) | TypeScript | Minesweeper in the terminal with flagging and difficulty levels. |
