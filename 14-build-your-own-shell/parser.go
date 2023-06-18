package main

// Redirect is one redirection on a command.
type Redirect struct {
	Type   TokenType // TokenRedirectOut, TokenRedirectAppend or TokenRedirectIn
	Target string
}

// Command is one stage of a pipeline.
type Command struct {
	Argv   []string
	Redirs []Redirect
}

// Pipeline is a list of commands joined by "|".
type Pipeline struct {
	Commands []Command
}

// Parse builds a Pipeline from tokens. An empty line gives an empty Pipeline.
func Parse(tokens []Token) (*Pipeline, error) {
	pl := &Pipeline{}
	if len(tokens) == 0 {
		return pl, nil
	}

	var stage []Token
	stages := [][]Token{}
	for _, t := range tokens {
		if t.Type == TokenPipe {
			stages = append(stages, stage)
			stage = nil
			continue
		}
		stage = append(stage, t)
	}
	stages = append(stages, stage)

	for _, s := range stages {
		cmd, err := parseStage(s)
		if err != nil {
			return nil, err
		}
		pl.Commands = append(pl.Commands, *cmd)
	}

	return pl, nil
}

func parseStage(tokens []Token) (*Command, error) {
	cmd := &Command{}

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch t.Type {
		case TokenWord:
			if t.Value == "&" {
				return nil, &SyntaxError{Msg: "background execution (&) is not supported"}
			}
			cmd.Argv = append(cmd.Argv, t.Value)

		case TokenRedirectOut, TokenRedirectAppend, TokenRedirectIn:
			if i+1 >= len(tokens) || tokens[i+1].Type != TokenWord {
				return nil, &SyntaxError{Msg: "missing target for redirection '" + t.Type.String() + "'"}
			}
			cmd.Redirs = append(cmd.Redirs, Redirect{Type: t.Type, Target: tokens[i+1].Value})
			i++

		default:
			return nil, &SyntaxError{Msg: "unexpected token '" + t.Value + "'"}
		}
	}

	if len(cmd.Argv) == 0 {
		return nil, &SyntaxError{Msg: "near '|': empty command"}
	}

	return cmd, nil
}
