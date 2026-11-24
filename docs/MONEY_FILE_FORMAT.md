# The Microsoft Money Data File Format

The canonical description of the format, and of the reader that implements it,
lives in the shared [money-engine module](../../money-engine): see
[money-engine: docs/MONEY_FILE_FORMAT.md](../../money-engine/docs/MONEY_FILE_FORMAT.md).

HB SunRise consumes the reader through
[`money-engine/msmoney`](../../money-engine/msmoney); this repository no longer
carries its own copy of the parser.
