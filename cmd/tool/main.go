package main

import (
	"os"
	"path/filepath"

	"github.com/hexdecteam/easegateway/cmd/tool/oplog"
	"github.com/hexdecteam/easegateway/pkg/common"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "Ease Gateway tool command line interface"
	app.Usage = ""
	app.Copyright = "(c) 2018 MegaEase.com"

	app.Commands = []cli.Command{
		oplogCmd,
	}

	app.Run(os.Args)
}

var oplogCmd = cli.Command{
	Name:  "oplog",
	Usage: "oplog interface",
	Subcommands: []cli.Command{
		{
			Name:  "retrieve",
			Usage: "Retrieve oplog operations from specified sequence",
			Flags: []cli.Flag{
				cli.Uint64Flag{
					Name:  "begin",
					Value: 1,
					Usage: "indicates begin sequence of retrieving oplog operations",
				},
				cli.Uint64Flag{
					Name:  "count",
					Value: 5,
					Usage: "indicates total count of retrieving oplog operations",
				},
				cli.StringFlag{
					Name:  "path",
					Value: filepath.Join(common.INVENTORY_HOME_DIR, "oplog"),
					Usage: "indicates oplog data path",
				},
			},
			Action: oplog.RetrieveOpLog,
		},
	},
}