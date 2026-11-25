export namespace main {
	
	export class accountView {
	    name: string;
	    currency: string;
	
	    static createFrom(source: any = {}) {
	        return new accountView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.currency = source["currency"];
	    }
	}
	export class categorySummaryView {
	    path: string;
	    count: number;
	    expense: number;
	    income: number;
	
	    static createFrom(source: any = {}) {
	        return new categorySummaryView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.count = source["count"];
	        this.expense = source["expense"];
	        this.income = source["income"];
	    }
	}
	export class columnView {
	    name: string;
	    type: string;
	    fixed: boolean;
	    number: number;
	    offset: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new columnView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.fixed = source["fixed"];
	        this.number = source["number"];
	        this.offset = source["offset"];
	        this.size = source["size"];
	    }
	}
	export class findingView {
	    severity: string;
	    kind: string;
	    account: string;
	    transferAccount: string;
	    time: string;
	    amount: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new findingView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.kind = source["kind"];
	        this.account = source["account"];
	        this.transferAccount = source["transferAccount"];
	        this.time = source["time"];
	        this.amount = source["amount"];
	        this.message = source["message"];
	    }
	}
	export class splitView {
	    date: string;
	    account: string;
	    category: string;
	    subCategory: string;
	    amount: number;
	    memo: string;
	
	    static createFrom(source: any = {}) {
	        return new splitView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.account = source["account"];
	        this.category = source["category"];
	        this.subCategory = source["subCategory"];
	        this.amount = source["amount"];
	        this.memo = source["memo"];
	    }
	}
	export class transferView {
	    date: string;
	    account: string;
	    transferAccount: string;
	    amount: number;
	    transferAmount: number;
	    matched: boolean;
	    memo: string;
	
	    static createFrom(source: any = {}) {
	        return new transferView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.account = source["account"];
	        this.transferAccount = source["transferAccount"];
	        this.amount = source["amount"];
	        this.transferAmount = source["transferAmount"];
	        this.matched = source["matched"];
	        this.memo = source["memo"];
	    }
	}
	export class transactionView {
	    time: string;
	    type: string;
	    account: string;
	    currency: string;
	    amount: number;
	    transferAccount: string;
	    transferAmount: number;
	    category: string;
	    subCategory: string;
	    memo: string;
	    split: boolean;
	
	    static createFrom(source: any = {}) {
	        return new transactionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.type = source["type"];
	        this.account = source["account"];
	        this.currency = source["currency"];
	        this.amount = source["amount"];
	        this.transferAccount = source["transferAccount"];
	        this.transferAmount = source["transferAmount"];
	        this.category = source["category"];
	        this.subCategory = source["subCategory"];
	        this.memo = source["memo"];
	        this.split = source["split"];
	    }
	}
	export class fileInfoView {
	    path: string;
	    name: string;
	    size: number;
	    sha256: string;
	
	    static createFrom(source: any = {}) {
	        return new fileInfoView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	    }
	}
	export class extractionView {
	    file: fileInfoView;
	    accounts: accountView[];
	    transactions: transactionView[];
	    categories: categorySummaryView[];
	    transfers: transferView[];
	    splits: splitView[];
	    skipped: number;
	    diagnostics: msmoney.ExtractedDiagnostics;
	    validation: findingView[];
	    elapsed: string;
	
	    static createFrom(source: any = {}) {
	        return new extractionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = this.convertValues(source["file"], fileInfoView);
	        this.accounts = this.convertValues(source["accounts"], accountView);
	        this.transactions = this.convertValues(source["transactions"], transactionView);
	        this.categories = this.convertValues(source["categories"], categorySummaryView);
	        this.transfers = this.convertValues(source["transfers"], transferView);
	        this.splits = this.convertValues(source["splits"], splitView);
	        this.skipped = source["skipped"];
	        this.diagnostics = this.convertValues(source["diagnostics"], msmoney.ExtractedDiagnostics);
	        this.validation = this.convertValues(source["validation"], findingView);
	        this.elapsed = source["elapsed"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class fieldView {
	    column: string;
	    type: string;
	    value: string;
	    null: boolean;
	
	    static createFrom(source: any = {}) {
	        return new fieldView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.column = source["column"];
	        this.type = source["type"];
	        this.value = source["value"];
	        this.null = source["null"];
	    }
	}
	
	
	export class tableView {
	    name: string;
	    definitionPage: number;
	    rowCount: number;
	    firstDataPage: number;
	    columns: columnView[];
	
	    static createFrom(source: any = {}) {
	        return new tableView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.definitionPage = source["definitionPage"];
	        this.rowCount = source["rowCount"];
	        this.firstDataPage = source["firstDataPage"];
	        this.columns = this.convertValues(source["columns"], columnView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class pageTypeCount {
	    type: number;
	    name: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new pageTypeCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.count = source["count"];
	    }
	}
	export class inspectionView {
	    file: fileInfoView;
	    version: number;
	    codePage: number;
	    languageID: number;
	    pageCount: number;
	    pageTypeCounts: pageTypeCount[];
	    tables: tableView[];
	
	    static createFrom(source: any = {}) {
	        return new inspectionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = this.convertValues(source["file"], fileInfoView);
	        this.version = source["version"];
	        this.codePage = source["codePage"];
	        this.languageID = source["languageID"];
	        this.pageCount = source["pageCount"];
	        this.pageTypeCounts = this.convertValues(source["pageTypeCounts"], pageTypeCount);
	        this.tables = this.convertValues(source["tables"], tableView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class rowsView {
	    table: string;
	    offset: number;
	    limit: number;
	    declaredRows: number;
	    rows: fieldView[][];
	    diagnostics: msmoney.ExtractedDiagnostics;
	
	    static createFrom(source: any = {}) {
	        return new rowsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.table = source["table"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	        this.declaredRows = source["declaredRows"];
	        this.rows = this.convertValues(source["rows"], fieldView);
	        this.diagnostics = this.convertValues(source["diagnostics"], msmoney.ExtractedDiagnostics);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

export namespace msmoney {
	
	export class ExtractedDiagnostics {
	    CorruptRows: number;
	    UnresolvedRows: number;
	    CorruptDefinitions: number;
	    UnknownCatalogEntries: number;
	    Samples: string[];
	
	    static createFrom(source: any = {}) {
	        return new ExtractedDiagnostics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CorruptRows = source["CorruptRows"];
	        this.UnresolvedRows = source["UnresolvedRows"];
	        this.CorruptDefinitions = source["CorruptDefinitions"];
	        this.UnknownCatalogEntries = source["UnknownCatalogEntries"];
	        this.Samples = source["Samples"];
	    }
	}

}

