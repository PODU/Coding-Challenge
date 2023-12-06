"""Enable ``python -m netmodel``."""

from __future__ import annotations

import sys

from netmodel.cli import main

if __name__ == "__main__":
    sys.exit(main())
