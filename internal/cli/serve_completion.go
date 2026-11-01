package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"my-agent/internal/i18n"
	"my-agent/internal/web"
)

func (e *env) cmdServe(ctx context.Context, args []string) error {
	fs := e.flags("serve")
	addr := fs.String("addr", "", i18n.T("cli.flag.addr"))
	if err := parse(fs, args); err != nil {
		return err
	}
	a, err := e.app()
	if err != nil {
		return err
	}
	if *addr == "" {
		*addr = a.Cfg.Server.Addr
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: web.New(a).Handler(), ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintln(e.out, i18n.T("cli.serve.listening", ln.Addr()))
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

const bashCompletion = `_my_agent() {
  local cur="${COMP_WORDS[COMP_CWORD]}" prev="${COMP_WORDS[COMP_CWORD-1]}"
  case "$prev" in
    config) COMPREPLY=($(compgen -W "show get set path validate" -- "$cur")); return;;
    providers) COMPREPLY=($(compgen -W "list test add" -- "$cur")); return;;
    rag) COMPREPLY=($(compgen -W "ingest search list remove clear" -- "$cur")); return;;
    models) COMPREPLY=($(compgen -W "list ps select use pull" -- "$cur")); return;;
    connectors) COMPREPLY=($(compgen -W "list add remove sync" -- "$cur")); return;;
    completion) COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "$cur")); return;;
  esac
  COMPREPLY=($(compgen -W "tui run agents mcp sessions init config providers chat ask rag models connectors serve completion version help" -- "$cur"))
}
complete -F _my_agent my-agent
`

const powershellCompletion = `Register-ArgumentCompleter -Native -CommandName my-agent -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $words = $commandAst.CommandElements | ForEach-Object { $_.ToString() }
  $sub = @{
    config     = 'show','get','set','path','validate'
    providers  = 'list','test','add'
    rag        = 'ingest','search','list','remove','clear'
    models     = 'list','ps','select','use','pull'
    connectors = 'list','add','remove','sync'
    completion = 'bash','zsh','fish','powershell'
  }
  $top = 'tui','run','agents','mcp','sessions','init','config','providers','chat','ask','rag','models','connectors','serve','completion','version','help'
  $options = $top
  foreach ($w in $words) { if ($sub.ContainsKey($w)) { $options = $sub[$w] } }
  $options | Where-Object { $_ -like "$wordToComplete*" } |
    ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
}
`

// zshCompletion reuses the bash function through bashcompinit.
const zshCompletion = `autoload -U +X compinit && compinit -C
autoload -U +X bashcompinit && bashcompinit
` + bashCompletion

const fishCompletion = `complete -c my-agent -f
complete -c my-agent -n __fish_use_subcommand -a "tui run agents mcp sessions init config providers chat ask rag models connectors serve completion version help"
complete -c my-agent -n "__fish_seen_subcommand_from config" -a "show get set path validate"
complete -c my-agent -n "__fish_seen_subcommand_from providers" -a "list test add"
complete -c my-agent -n "__fish_seen_subcommand_from rag" -a "ingest search list remove clear"
complete -c my-agent -n "__fish_seen_subcommand_from models" -a "list ps select use pull"
complete -c my-agent -n "__fish_seen_subcommand_from connectors" -a "list add remove sync"
complete -c my-agent -n "__fish_seen_subcommand_from sessions" -a "list remove"
complete -c my-agent -n "__fish_seen_subcommand_from completion" -a "bash zsh fish powershell"
complete -c my-agent -l agent -r -d agent
complete -c my-agent -l model -r -d model
complete -c my-agent -l continue
complete -c my-agent -l lang -r -a "auto en pt-BR es"
`

func (e *env) cmdCompletion(args []string) error {
	if len(args) != 1 {
		return usageSyntax("completion bash|zsh|fish|powershell")
	}
	switch args[0] {
	case "bash":
		fmt.Fprint(e.out, bashCompletion)
	case "zsh":
		fmt.Fprint(e.out, zshCompletion)
	case "fish":
		fmt.Fprint(e.out, fishCompletion)
	case "powershell", "pwsh":
		fmt.Fprint(e.out, powershellCompletion)
	default:
		return usageKey("cli.completion.unsupported", args[0])
	}
	return nil
}
