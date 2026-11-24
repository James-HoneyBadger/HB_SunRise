// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

package errs

import "net/http"

// Errors reported while reading a Microsoft Money data file (.mny) or backup
// (.mbf). The categories and codes match honey-ledger so messages stay stable.
var (
	ErrImportFileTypeNotSupported                  = NewNormalError(NormalSubcategoryTransaction, 26, http.StatusBadRequest, "import file type not supported")
	ErrImportMicrosoftMoneyDataFileNotReadable     = NewNormalError(NormalSubcategoryTransaction, 41, http.StatusBadRequest, "microsoft money data file cannot be read directly")
	ErrImportMicrosoftMoneyDataFileEncrypted       = NewNormalError(NormalSubcategoryTransaction, 43, http.StatusBadRequest, "microsoft money data file is password protected")
	ErrImportMicrosoftMoneyBackupFileNotReadable   = NewNormalError(NormalSubcategoryTransaction, 44, http.StatusBadRequest, "microsoft money backup file cannot be read")
	ErrImportMicrosoftMoneyBackupFileTooLarge      = NewNormalError(NormalSubcategoryTransaction, 45, http.StatusBadRequest, "microsoft money backup file is too large")
)
