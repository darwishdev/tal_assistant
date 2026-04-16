export namespace adkutils {
	
	export class EvaluationCriteria {
	    bonus_points: string;
	    must_mention: string;
	
	    static createFrom(source: any = {}) {
	        return new EvaluationCriteria(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bonus_points = source["bonus_points"];
	        this.must_mention = source["must_mention"];
	    }
	}
	export class FollowupTrigger {
	    condition: string;
	    follow_up: string;
	
	    static createFrom(source: any = {}) {
	        return new FollowupTrigger(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.condition = source["condition"];
	        this.follow_up = source["follow_up"];
	    }
	}
	export class QuestionBankQuestion {
	    id: string;
	    category: string;
	    difficulty: string;
	    estimated_time_minutes: number;
	    evaluation_criteria: EvaluationCriteria[];
	    followup_triggers: FollowupTrigger[];
	    ideal_answer_keywords: string;
	    pass_treshold: number;
	    question: string;
	
	    static createFrom(source: any = {}) {
	        return new QuestionBankQuestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.difficulty = source["difficulty"];
	        this.estimated_time_minutes = source["estimated_time_minutes"];
	        this.evaluation_criteria = this.convertValues(source["evaluation_criteria"], EvaluationCriteria);
	        this.followup_triggers = this.convertValues(source["followup_triggers"], FollowupTrigger);
	        this.ideal_answer_keywords = source["ideal_answer_keywords"];
	        this.pass_treshold = source["pass_treshold"];
	        this.question = source["question"];
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

export namespace atsclient {
	
	export class CandidateProject {
	    name: string;
	    description: string[];
	
	    static createFrom(source: any = {}) {
	        return new CandidateProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}
	export class CandidateEducation {
	    degree: string;
	    institution: string;
	    year: string;
	
	    static createFrom(source: any = {}) {
	        return new CandidateEducation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.degree = source["degree"];
	        this.institution = source["institution"];
	        this.year = source["year"];
	    }
	}
	export class CandidateExperience {
	    company: string;
	    role: string;
	    from: string;
	    to: string;
	    responsibilities: string[];
	
	    static createFrom(source: any = {}) {
	        return new CandidateExperience(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.company = source["company"];
	        this.role = source["role"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.responsibilities = source["responsibilities"];
	    }
	}
	export class Candidate {
	    name: string;
	    email: string;
	    phone: string;
	    designation?: string;
	    summary: string;
	    skills: string[];
	    experience: CandidateExperience[];
	    education: CandidateEducation[];
	    projects: CandidateProject[];
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	        this.phone = source["phone"];
	        this.designation = source["designation"];
	        this.summary = source["summary"];
	        this.skills = source["skills"];
	        this.experience = this.convertValues(source["experience"], CandidateExperience);
	        this.education = this.convertValues(source["education"], CandidateEducation);
	        this.projects = this.convertValues(source["projects"], CandidateProject);
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
	
	
	
	export class EvaluationCriteria {
	    must_mention: string[];
	    bonus_points: string[];
	
	    static createFrom(source: any = {}) {
	        return new EvaluationCriteria(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.must_mention = source["must_mention"];
	        this.bonus_points = source["bonus_points"];
	    }
	}
	export class ExpectedSkill {
	    skill: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ExpectedSkill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skill = source["skill"];
	        this.description = source["description"];
	    }
	}
	export class FollowupTrigger {
	    condition: string;
	    follow_up: string;
	
	    static createFrom(source: any = {}) {
	        return new FollowupTrigger(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.condition = source["condition"];
	        this.follow_up = source["follow_up"];
	    }
	}
	export class InterviewDetail {
	    id: string;
	    scheduled_on: string;
	    designation?: string;
	    expected_average_rating: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new InterviewDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.scheduled_on = source["scheduled_on"];
	        this.designation = source["designation"];
	        this.expected_average_rating = source["expected_average_rating"];
	        this.status = source["status"];
	    }
	}
	export class Question {
	    id: string;
	    category: string;
	    difficulty: string;
	    estimated_time_minutes: number;
	    question: string;
	    ideal_answer_keywords: string[];
	    evaluation_criteria: EvaluationCriteria[];
	    followup_triggers: FollowupTrigger[];
	    pass_threshold: number;
	
	    static createFrom(source: any = {}) {
	        return new Question(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.difficulty = source["difficulty"];
	        this.estimated_time_minutes = source["estimated_time_minutes"];
	        this.question = source["question"];
	        this.ideal_answer_keywords = source["ideal_answer_keywords"];
	        this.evaluation_criteria = this.convertValues(source["evaluation_criteria"], EvaluationCriteria);
	        this.followup_triggers = this.convertValues(source["followup_triggers"], FollowupTrigger);
	        this.pass_threshold = source["pass_threshold"];
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
	export class QuestionBank {
	    name: string;
	    focus_areas: string[];
	    questions: Question[];
	
	    static createFrom(source: any = {}) {
	        return new QuestionBank(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.focus_areas = source["focus_areas"];
	        this.questions = this.convertValues(source["questions"], Question);
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
	export class InterviewRound {
	    name: string;
	    type: string;
	    designation: string;
	    expected_average_rating: number;
	    expected_skills: ExpectedSkill[];
	
	    static createFrom(source: any = {}) {
	        return new InterviewRound(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.designation = source["designation"];
	        this.expected_average_rating = source["expected_average_rating"];
	        this.expected_skills = this.convertValues(source["expected_skills"], ExpectedSkill);
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
	export class PipelineStep {
	    code: string;
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new PipelineStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class JobDescriptionSection {
	    title: string;
	    description?: string;
	    points: string[];
	
	    static createFrom(source: any = {}) {
	        return new JobDescriptionSection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.description = source["description"];
	        this.points = source["points"];
	    }
	}
	export class Job {
	    id: string;
	    title: string;
	    designation: string;
	    department: string;
	    location: string;
	    description: JobDescriptionSection[];
	    current_pipeline_step: PipelineStep;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.designation = source["designation"];
	        this.department = source["department"];
	        this.location = source["location"];
	        this.description = this.convertValues(source["description"], JobDescriptionSection);
	        this.current_pipeline_step = this.convertValues(source["current_pipeline_step"], PipelineStep);
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
	export class InterviewFindResult {
	    interview: InterviewDetail;
	    candidate: Candidate;
	    job: Job;
	    round: InterviewRound;
	    question_bank: QuestionBank;
	
	    static createFrom(source: any = {}) {
	        return new InterviewFindResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.interview = this.convertValues(source["interview"], InterviewDetail);
	        this.candidate = this.convertValues(source["candidate"], Candidate);
	        this.job = this.convertValues(source["job"], Job);
	        this.round = this.convertValues(source["round"], InterviewRound);
	        this.question_bank = this.convertValues(source["question_bank"], QuestionBank);
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
	export class InterviewListItem {
	    name: string;
	    status: string;
	    scheduled_on: string;
	    from_time: string;
	    to_time: string;
	    interview_round: string;
	    job_applicant: string;
	    candidate_name: string;
	    candidate_email: string;
	    job_opening?: string;
	    job_title?: string;
	
	    static createFrom(source: any = {}) {
	        return new InterviewListItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.scheduled_on = source["scheduled_on"];
	        this.from_time = source["from_time"];
	        this.to_time = source["to_time"];
	        this.interview_round = source["interview_round"];
	        this.job_applicant = source["job_applicant"];
	        this.candidate_name = source["candidate_name"];
	        this.candidate_email = source["candidate_email"];
	        this.job_opening = source["job_opening"];
	        this.job_title = source["job_title"];
	    }
	}
	
	
	
	export class LoginResponse {
	    message: string;
	    home_page: string;
	    full_name: string;
	
	    static createFrom(source: any = {}) {
	        return new LoginResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.home_page = source["home_page"];
	        this.full_name = source["full_name"];
	    }
	}
	
	

}

export namespace main {
	
	export class DeviceSource {
	    ID: string;
	    Name: string;
	    IsDefault: boolean;
	    IsPersonal: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeviceSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.IsDefault = source["IsDefault"];
	        this.IsPersonal = source["IsPersonal"];
	    }
	}
	export class ListSourcesResonse {
	    Mics: DeviceSource[];
	    Speakers: DeviceSource[];
	    Screens: recording.ScreenSource[];
	
	    static createFrom(source: any = {}) {
	        return new ListSourcesResonse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Mics = this.convertValues(source["Mics"], DeviceSource);
	        this.Speakers = this.convertValues(source["Speakers"], DeviceSource);
	        this.Screens = this.convertValues(source["Screens"], recording.ScreenSource);
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

export namespace recording {
	
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

