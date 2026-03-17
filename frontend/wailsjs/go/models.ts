export namespace ffmpeg {
	
	export class ScreenSource {
	    ID: string;
	    Name: string;
	    OffsetX: number;
	    OffsetY: number;
	    Width: number;
	    Height: number;
	    Screenshot: string;
	
	    static createFrom(source: any = {}) {
	        return new ScreenSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.OffsetX = source["OffsetX"];
	        this.OffsetY = source["OffsetY"];
	        this.Width = source["Width"];
	        this.Height = source["Height"];
	        this.Screenshot = source["Screenshot"];
	    }
	}

}

export namespace main {
	
	export class DeviceSource {
	    ID: string;
	    Name: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	    }
	}
	export class ListSourcesResonse {
	    Mics: DeviceSource[];
	    Speakers: DeviceSource[];
	    Screens: ffmpeg.ScreenSource[];
	
	    static createFrom(source: any = {}) {
	        return new ListSourcesResonse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Mics = this.convertValues(source["Mics"], DeviceSource);
	        this.Speakers = this.convertValues(source["Speakers"], DeviceSource);
	        this.Screens = this.convertValues(source["Screens"], ffmpeg.ScreenSource);
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

