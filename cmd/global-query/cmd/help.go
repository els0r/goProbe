package cmd

var supportedCmds = "{server}"

var helpBase = `
  global-query ` + supportedCmds + `

  Query server for running distributed goQuery queries and aggregating the results.
`

var helpBaseLong = helpBase + `
  Meant to run in server mode via the "server" command.
`
