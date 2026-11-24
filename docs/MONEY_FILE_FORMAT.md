# The Microsoft Money Data File Format

A Microsoft Money file is a single-file database. Its header names the **MSISAM**
engine rather than the Access engine, and that rename is the whole reason a Jet
or Access driver refuses it: the page layout is Jet 4 with a few structures only
Money writes.

Everything below is what the reader in
[`pkg/converters/msmoney/msisam`](../pkg/converters/msmoney/msisam) actually
decodes. Offsets are hexadecimal and page-relative; every integer is little
endian unless it says otherwise.

## Containers

| Extension | Contents |
| --- | --- |
| `.mny` | The database itself |
| `.mbf` | A Microsoft Cabinet (`MSCF`) holding one `.mny` member |

A backup is unpacked in memory and then read exactly like a `.mny`. Money does
not name the cabinet member after the file it holds, so the member is recognised
by the `MSISAM Database` string at offset 4 rather than by its name.

The unpacking is bounded, because a cabinet is attacker-controlled input: at most
32 members are examined, a member that declares more than 512 MiB is refused
before it is decompressed, and a member that expands past that limit is refused
afterwards. A cabinet that is damaged, holds no Money database, or breaks either
bound is reported as `ErrImportMicrosoftMoneyBackupFileNotReadable` (or
`...TooLarge`), never as a partial import.

## Where the format stops

| Money version | Support |
| --- | --- |
| 2001 through Money Plus 2008, including Money Plus Sunset | Read: this is the 4096-byte Jet 4 layout below |
| 2000 and earlier | Not read: those files use a 2048-byte page layout |
| Password protected | Not read: reported as `ErrImportMicrosoftMoneyDataFileEncrypted` |

## The header page

Page 0 is the header page. The fields the reader decodes are:

| Offset | Size | Field |
| --- | --- | --- |
| `0x00` | 1 | Page type, `0x00` |
| `0x04` | 15 | Engine name, `MSISAM Database` |
| `0x14` | 1 | Engine version, `1` for the Jet 4 layout |
| `0x18` | 128 | Scrambled block, described below |

The engine name is what `IsMicrosoftMoneyDataFile` tests, and it is why no
Access driver will open the file: the drivers look for `Standard Jet DB`.

### The scrambled block and the database key

The 128 bytes from `0x18` are encrypted with **RC4 keyed by `C7 DA 39 6B`**
whether or not the database has a password. A reader has to decrypt them before
the fields inside mean anything:

| Offset | Size | Field |
| --- | --- | --- |
| `0x3C` | 2 | Windows code page of the non-Unicode data, typically 1252 |
| `0x3E` | 4 | Database key |
| `0x6E` | 2 | Windows language identifier, typically 1033 |

RC4 is symmetric, so the same call both hides and restores the block; the reader
keeps one `rc4` function and uses it in one direction.

A **non-zero database key means the database is password protected**. In that
case RC4 scrambles every page with a key derived from the password, and no amount
of header parsing will produce sensible rows. The reader checks the key before it
touches any other page and returns `ErrEncryptedDatabase`, which the command
layer turns into "this file is password protected" — not into a pile of garbage
transactions.

Two smaller facts about the header:

- **Page count is rounded down.** Money appends trailing data, so a database can
  end with a partial page. Only whole 4096-byte pages are addressable
  (`PageCount = len(data) / 4096`).
- **A file shorter than one page is not a database** (`ErrFileTooSmall`).

## Page types

The first byte of every page is its type:

| Value | Type | Holds |
| --- | --- | --- |
| `0x00` | Header | Page 0 |
| `0x01` | Data | Table rows |
| `0x02` | Definition | Table definitions |
| `0x03` | Index | Index pages (never read) |
| `0x04` | Leaf | The overflow of a long value |
| `0x05` | Usage map | Which pages a table owns (never read) |

Offset `0x04` of a page is a 32-bit page number whose meaning depends on the
type:

| Page type | Offset `0x04` names |
| --- | --- |
| Data | The definition page of the table whose rows it holds |
| Definition | The page that continues this definition, or 0 |
| Leaf | The page that continues this long value, or 0 |

The reader uses that one field twice: to group a table's data pages
(`dataPagesOf`) and to follow a definition larger than one page (`pageChain`).

## Table definitions

The engine has no table of tables. Every definition page describes one table,
and the pages are found by walking the file and keeping those whose type byte is
`0x02`. Continuation pages carry the same type byte, so the reader collects the
pages first and removes any page another definition points at; what remains is
the set of definitions.

A definition page begins with:

| Offset | Size | Field |
| --- | --- | --- |
| `0x04` | 4 | Page that continues the definition, or 0 |
| `0x10` | 4 | Row count |
| `0x2B` | 2 | Number of variable length columns |
| `0x2D` | 2 | Number of columns |
| `0x33` | 4 | Number of real indexes |
| `0x38` | 2 | First data page |
| `0x3F` | | Start of the column entries |

The column entries are 25 bytes each and begin at
`0x3F + realIndexes × 12`, because the index entries sit between the header and
the columns:

| Offset | Size | Field |
| --- | --- | --- |
| `0x00` | 1 | Column type |
| `0x05` | 2 | Column number, also its bit in the row's null mask |
| `0x07` | 2 | Index into the row's variable column offset table |
| `0x0B` | 1 | Scale |
| `0x0C` | 1 | Precision |
| `0x0F` | 1 | Flags; bit `0x01` marks a fixed width column |
| `0x15` | 2 | Byte offset of a fixed width column within the row |
| `0x17` | 2 | Width of a fixed column, or maximum length of a variable column |

The names follow the entries, in column order: a 16-bit byte length, then that
many bytes of text.

A definition larger than one page continues on the page named at `0x04`. A
continuation page repeats the eight-byte page header, so its content starts at
offset 8.

### Column types

| Value | Type | Stored as |
| --- | --- | --- |
| `0x01` | Bool | The null mask bit itself; it has no width |
| `0x02` | Byte | 1 byte |
| `0x03` | Int | 2 bytes, signed |
| `0x04` | LongInt | 4 bytes, signed |
| `0x05` | Money | 8 bytes, signed, scaled by 10 000 |
| `0x06` | Float | 4 bytes, IEEE 754 |
| `0x07` | Double | 8 bytes, IEEE 754 |
| `0x08` | DateTime | 8 bytes, IEEE 754, days since 1899-12-30 |
| `0x09` | Binary | Raw bytes |
| `0x0A` | Text | UTF-16, compressed as described below |
| `0x0B` | OLE | A long value, returned as bytes |
| `0x0C` | Memo | A long value, returned as text |
| `0x0F` | GUID | 16 bytes, first three fields little endian |
| `0x10` | Numeric | A sign byte and a 16-byte little endian magnitude |
| `0x12` | Complex | Not decoded |

An unknown type code makes the definition corrupt rather than silently
unreadable: guessing at a width would misalign every field after it.

### Bounds

Each of these exists because the corresponding number comes from the file:

| Bound | Value | Why |
| --- | --- | --- |
| Column count | 1 to 1024 | Bounds the walk over column entries |
| Real index count | at most 1024 | Bounds the offset into the entries |
| Continuation pages per definition | 64 | Stops a cycle in the page chain |
| Row count | capped at 1 048 576 | The count only sizes a buffer |

## Rows

A data page holds its row count at `0x0C`, then one 16-bit start offset per row
from `0x0E`. Rows are filled from the end of the page backwards, so row 0 starts
highest and the entry for row *n* doubles as the end of row *n−1*.

The high bits of a start offset are flags:

| Bit | Meaning |
| --- | --- |
| `0x8000` | The row is deleted |
| `0x4000` | The slot holds a 4-byte page and row pointer to the real row |
| `0x3FFF` | Mask for the offset itself |

A relocated row stores `page << 8 | slot`; the reader resolves it and cracks the
row where it really lives.

A row is laid out as:

1. the column count, 2 bytes;
2. the fixed width columns, each at `2 + offset` from the given column entry;
3. the variable length columns, packed in order;
4. growing down from the end of the row: the offset table of the variable
   columns (one more entry than there are variable columns, written highest
   first), then the count of variable columns, then the not-null mask.

A bit set in the not-null mask means the column holds a value. A bool is the
exception: its value *is* the bit, so it is never null. A column whose bit is
clear still occupies its fixed slot, which is why the slot counter advances for
null columns too.

## Value encodings

### Text

Text is UTF-16, but the engine compresses it. A `0x00` byte toggles between two
modes, beginning in compressed mode:

- **compressed**: each byte is one character with a zero high byte (ASCII
  therefore takes half the space);
- **plain**: two bytes per character.

A reader expands the toggles back to UTF-16 bytes and decodes from there. A memo
that fails to decode is skipped, not allowed to abort the file.

### Money and amounts

Money is an 8-byte signed integer scaled by **10 000** — four decimal places.
The importer keeps two, rounding away from zero, because that is the scale of
the ledgers it writes. Nothing in the file says how many decimals a particular
amount used, so the fourth place is dropped deliberately rather than truncated
towards zero.

### Dates

A DateTime is an IEEE 754 double counting **days from 1899-12-30**, so the
fractional part is the time of day. An unset date is a value far outside any
real range; the importer keeps only years 1900 to 2200 and treats anything else
as no date, which makes the row un-importable and counted as skipped.

### Numeric

A sign byte (`0x80` set means negative) followed by a 16-byte little endian
magnitude, divided by 10 once per unit of the column's scale.

### GUID

16 bytes: the first three fields are little endian 32/16/16-bit numbers, the last
eight bytes are byte order. Rendered dashed, upper case.

### Long values

Memo and OLE columns store a **12-byte header** in the row instead of the value:

| Offset | Size | Field |
| --- | --- | --- |
| `0x00` | 4 | Length, with flags in the high bits |
| `0x04` | 4 | Page and row of the first leaf page |

| Test | Meaning |
| --- | --- |
| bit 31 set | Inline: the value follows the header |
| bit 30 set | One leaf page: `0x04` names it |
| high byte zero | A chain of leaf pages; `0x04` names the first |
| anything else | Unknown flags, reported as `ErrCorruptRow` |

In a chain, every chunk starts with the page and row of the next chunk, so the
walk follows `0x00` of each chunk. The chain is followed for at most 64 pages,
and the claimed length is not trusted: the buffer grows with the data actually
found and is trimmed afterwards, so a header claiming 4 GiB cannot ask for 4 GiB.

## The catalog

The catalog is the table registered under definition page 2, though page 2
itself does not decode. Its rows hold the definition page of an object first and
the object's name first among the variable columns.

A real Money database only names part of its objects — often the tail of the
alphabet — so the rest are named from their **identity column**: the engine calls
the first column `h` followed by the lower case table name, so the table whose
first column is `hacct` is `ACCT`. A derived name is used only when exactly one
table claims it; otherwise the table is reported as `UNKNOWN_<definition page>`
rather than given a plausible but wrong name.

Money names its core tables:

| Table | Holds |
| --- | --- |
| `ACCT` | Accounts: `hacct`, `hcrnc`, `szFull` |
| `CAT` | Categories: `hcat`, `hcatParent`, `szFull` |
| `CRNC` | Currencies: `hcrnc`, `szIsoCode` |
| `TRN` | Transactions: `htrn`, `hacct`, `hacctLink`, `hcat`, `hsec`, `amt`, `mMemo`, `dt` |
| `TRN_SPLIT` | Splits: `htrnParent`, and the links to the split rows |

A file without a `CRNC` table is read with no currency, because older files name
currencies differently; a missing `TRN_SPLIT` simply means no splits. A missing
`ACCT`, `CAT` or `TRN` makes the file unreadable.

## How a ledger is recovered

The tables are a relational model; a ledger is a list of transactions. Four
shapes in the data have no direct equivalent:

### A transfer is stored twice

One row sits in the account the money left and one in the account that received
it, and `hacctLink` names the other account on both. Importing both would double
every transfer, so only the row with the negative amount becomes a transfer; the
receiving row is counted as skipped.

### A split transaction is a parent plus its parts

The parent row holds the total and one `TRN` row holds each part, linked back
through `TRN_SPLIT.htrnParent`. The parent is dropped — it would double the
money — and every part becomes a transaction in the parent's account and on its
date.

### An investment row names a security

If `hsec` is non-zero the row records shares, not money. It is skipped; the cash
side of the activity, which is an ordinary `TRN` row without `hsec`, still
imports.

### Categories are a tree with two roots

`INCOME` and `EXPENSE` are directions rather than categories. A category whose
parent handle is 0 is a root and is not imported as a category; a transaction
filed straight against a root stays uncategorized, and the categories beneath a
root are not grouped under it. For everything else the parent is the grouping and
the category itself is the subcategory.

### What a skipped row is

A row is counted as skipped and reported to the user, never silently dropped,
when it is: a split total, an investment row, the receiving side of a transfer, a
row with no amount, a row whose account is unknown, or a row with no usable date.
`Skipped` is a single count because the user's question is "how much is missing",
not "which of six reasons".

## Empirical status

- The **committed** tests build a valid database and a valid `.mbf` in memory
  (`pkg/converters/msmoney/synthetic`), so the pipeline runs end to end without a
  file Microsoft's licence does not allow redistributing. That generator is a
  second, independent implementation of the layout above: the strongest test
  that this document and the reader agree.
- The tests that read a **real** database run only when `MSISAM_SAMPLE_FILE`
  points at one, and then check that the header is version 1, that the file holds
  more than a hundred pages of the expected types, and that the core tables
  exist.
- Money's own acceptance is still the final word. A file this reader accepts is
  evidence the reader is not wrong about the numbers it prints; it is not proof
  that Money would accept it as its own, which is what the reconstruction work in
  [BUILDING_HB_SUNRISE.md](BUILDING_HB_SUNRISE.md) has to earn.
