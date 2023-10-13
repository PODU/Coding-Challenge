; A closure that captures and mutates its own private state.
(defun make-counter ()
  (let ((count 0))
    (lambda ()
      (set! count (+ count 1))
      count)))

(define c (make-counter))
(format t "~A~%" (c))
(format t "~A~%" (c))
(format t "~A~%" (c))
