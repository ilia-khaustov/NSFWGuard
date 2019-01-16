# NSFWGuard

## About

NSFWGuard is a project that wraps [NSFW API](https://github.com/rahiel/open_nsfw--) into a [Telegram bot](https://core.telegram.org/bots) known as [NSFWGuard](https://t.me/NSFWGuardBot). Using this repository one can easily launch a similar bot with whatever modifications needed for a specific purpose.

## Requirements

1. Docker with docker-compose have to be installed and available in your system.
2. Build open_nsfw image: `docker build -t open_nsfw https://raw.githubusercontent.com/rahiel/open_nsfw--/master/Dockerfile`

## Setup for Development

0. Make sure you have your Telegram bot token obtained from the [BotFather](https://telegram.me/botfather).
1. Create `.env` file in a project root:
```
TLGRM_TOKEN=paste-your-bot-token-here
NSFW_API_ADDR=http://nsfwapi:8080
NSFW_API_PREC=0.95
```
2. Run `docker-compose up` in a project root.
3. Wait a bit until process stops producing logs...

Bot service should be up and running by now; any changes in `./NSFWGuard` dir will trigger restart of the service. Also, new dependencies will be installed if found by `go get ./...` cmd.
