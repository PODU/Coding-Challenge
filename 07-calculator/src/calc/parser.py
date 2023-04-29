"""Recursive-descent (precedence-climbing) parser producing an AST.

The grammar (EBNF)::

    expression := term      { ("+" | "-") term }      // precedence 1, left
    term       := factor    { ("*" | "/") factor }    // precedence 2, left
    factor     := unary     [ "^" factor ]            // precedence 3, right
    unary      := ("+" | "-") unary | primary         // precedence 4, prefix
    primary    := NUMBER
                | IDENT                                // constant
                | IDENT "(" expression ")"             // function call
                | "(" expression ")"

``^`` is right-associative because ``factor`` recurses on its right side.
"""

from __future__ import annotations

from decimal import Decimal, InvalidOperation

from .ast_nodes import Binary, Call, Const, Expr, Num, Unary
from .errors import ParseError
from .tokens import Token, TokenType


def parse(tokens: list[Token]) -> Expr:
    return _Parser(tokens).parse()


class _Parser:
    def __init__(self, tokens: list[Token]) -> None:
        self._tokens = tokens
        self._i = 0

    @property
    def _current(self) -> Token:
        return self._tokens[self._i]

    def _advance(self) -> Token:
        tok = self._tokens[self._i]
        if tok.type is not TokenType.EOF:
            self._i += 1
        return tok

    def _check(self, ttype: TokenType) -> bool:
        return self._current.type is ttype

    def _match(self, *types: TokenType) -> Token | None:
        if self._current.type in types:
            return self._advance()
        return None

    def _expect(self, ttype: TokenType, what: str) -> Token:
        if self._current.type is ttype:
            return self._advance()
        raise ParseError(
            f"expected {what}, found {self._describe(self._current)}",
            pos=self._current.pos,
        )

    @staticmethod
    def _describe(tok: Token) -> str:
        if tok.type is TokenType.EOF:
            return "end of input"
        return repr(tok.lexeme)

    def parse(self) -> Expr:
        if self._check(TokenType.EOF):
            raise ParseError("empty expression", pos=self._current.pos)
        expr = self._expression()
        if not self._check(TokenType.EOF):
            tok = self._current
            raise ParseError(
                f"unexpected {self._describe(tok)}", pos=tok.pos
            )
        return expr

    def _expression(self) -> Expr:
        expr = self._term()
        while True:
            tok = self._match(TokenType.PLUS, TokenType.MINUS)
            if tok is None:
                return expr
            right = self._term()
            expr = Binary(tok.lexeme, expr, right, pos=tok.pos)

    def _term(self) -> Expr:
        expr = self._factor()
        while True:
            tok = self._match(TokenType.STAR, TokenType.SLASH)
            if tok is None:
                return expr
            right = self._factor()
            expr = Binary(tok.lexeme, expr, right, pos=tok.pos)

    def _factor(self) -> Expr:
        base = self._unary()
        tok = self._match(TokenType.CARET)
        if tok is None:
            return base
        exponent = self._factor()
        return Binary(tok.lexeme, base, exponent, pos=tok.pos)

    def _unary(self) -> Expr:
        tok = self._match(TokenType.PLUS, TokenType.MINUS)
        if tok is not None:
            operand = self._unary()
            return Unary(tok.lexeme, operand, pos=tok.pos)
        return self._primary()

    def _primary(self) -> Expr:
        tok = self._current

        if tok.type is TokenType.NUMBER:
            self._advance()
            try:
                value = Decimal(tok.lexeme)
            except InvalidOperation as exc:  # pragma: no cover - lexer guards this
                raise ParseError(
                    f"malformed number {tok.lexeme!r}", pos=tok.pos
                ) from exc
            return Num(value, pos=tok.pos)

        if tok.type is TokenType.IDENT:
            self._advance()
            if self._check(TokenType.LPAREN):
                self._advance()
                arg = self._expression()
                self._expect(TokenType.RPAREN, "')'")
                return Call(tok.lexeme, arg, pos=tok.pos)
            return Const(tok.lexeme, pos=tok.pos)

        if tok.type is TokenType.LPAREN:
            self._advance()
            if self._check(TokenType.RPAREN):
                raise ParseError("empty parentheses", pos=tok.pos)
            inner = self._expression()
            self._expect(TokenType.RPAREN, "')'")
            return inner

        if tok.type is TokenType.RPAREN:
            raise ParseError("unbalanced parentheses", pos=tok.pos)

        raise ParseError(
            f"expected a number, found {self._describe(tok)}", pos=tok.pos
        )
