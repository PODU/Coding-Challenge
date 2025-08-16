( FizzBuzz for 1..15, space-separated on a single line.            )
( Nested IF inside a DO loop inside a colon definition.            )

: fizzbuzz ( -- )
  16 1 do
    i 15 mod 0 = if ." FizzBuzz "
    else i 3 mod 0 = if ." Fizz "
    else i 5 mod 0 = if ." Buzz "
    else i .
    then then then
  loop
;

fizzbuzz
cr
