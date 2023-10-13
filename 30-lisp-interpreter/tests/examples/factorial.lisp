; Factorial via straightforward recursion.
(defun fact (n)
  (if (<= n 1)
      1
      (* n (fact (- n 1)))))

(format t "5! = ~A~%" (fact 5))
(format t "10! = ~A~%" (fact 10))
