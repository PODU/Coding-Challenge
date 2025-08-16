( Fibonacci: print the first N Fibonacci numbers, space-separated. )
( Strategy: keep the pair  a b  on the stack; each step prints a and )
( advances to  b a+b .  See README for the stack-effect walk-through.  )

: fibs ( n -- )
  0 1 rot 0 do          ( seed a=0 b=1, then  n 0 do )
    over .              ( print a )
    swap over +         ( a b -- b a+b )
  loop
  drop drop             ( discard the leftover pair )
;

10 fibs
cr
