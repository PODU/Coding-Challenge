from __future__ import annotations


class BgRemoveError(Exception):
    """Base class for errors this tool reports to the user."""


class InvalidImageError(BgRemoveError):
    """The file is missing, not an image, or truncated/corrupt."""


class UnsupportedFormatError(InvalidImageError):
    """The file extension is not JPG or PNG."""


class OutputCollisionError(BgRemoveError):
    """The output path is the same as an input."""
