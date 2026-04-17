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
	    order: number;
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
	        this.order = source["order"];
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
	
	
	
	export class DriveAuthStatus {
	    status: string;
	    auth_url?: string;
	
	    static createFrom(source: any = {}) {
	        return new DriveAuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.auth_url = source["auth_url"];
	    }
	}
	export class DriveUploadResult {
	    path: string;
	    file_id: string;
	    file_url: string;
	
	    static createFrom(source: any = {}) {
	        return new DriveUploadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.file_id = source["file_id"];
	        this.file_url = source["file_url"];
	    }
	}
	export class DriveUploadFolderResponse {
	    status: string;
	    auth_url?: string;
	    tal_folder_id?: string;
	    session_folder_id?: string;
	    session_folder_url?: string;
	    uploaded_count?: number;
	    uploaded?: DriveUploadResult[];
	
	    static createFrom(source: any = {}) {
	        return new DriveUploadFolderResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.auth_url = source["auth_url"];
	        this.tal_folder_id = source["tal_folder_id"];
	        this.session_folder_id = source["session_folder_id"];
	        this.session_folder_url = source["session_folder_url"];
	        this.uploaded_count = source["uploaded_count"];
	        this.uploaded = this.convertValues(source["uploaded"], DriveUploadResult);
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
	
	export class AppLoginResponse {
	    ats_login?: atsclient.LoginResponse;
	    member?: workableclient.Member;
	
	    static createFrom(source: any = {}) {
	        return new AppLoginResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ats_login = this.convertValues(source["ats_login"], atsclient.LoginResponse);
	        this.member = this.convertValues(source["member"], workableclient.Member);
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

export namespace workableclient {
	
	export class Location {
	    location_str?: string;
	    country?: string;
	    country_code?: string;
	    region?: string;
	    region_code?: string;
	    city?: string;
	    zip_code?: string;
	    telecommuting?: boolean;
	    workplace_type?: string;
	
	    static createFrom(source: any = {}) {
	        return new Location(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.location_str = source["location_str"];
	        this.country = source["country"];
	        this.country_code = source["country_code"];
	        this.region = source["region"];
	        this.region_code = source["region_code"];
	        this.city = source["city"];
	        this.zip_code = source["zip_code"];
	        this.telecommuting = source["telecommuting"];
	        this.workplace_type = source["workplace_type"];
	    }
	}
	export class SocialProfile {
	    type: string;
	    name: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new SocialProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.url = source["url"];
	    }
	}
	export class ExperienceEntry {
	    id: string;
	    title?: string;
	    summary?: string;
	    start_date?: string;
	    end_date?: string;
	    company?: string;
	    industry?: string;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExperienceEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.start_date = source["start_date"];
	        this.end_date = source["end_date"];
	        this.company = source["company"];
	        this.industry = source["industry"];
	        this.current = source["current"];
	    }
	}
	export class EducationEntry {
	    id: string;
	    degree?: string;
	    school?: string;
	    field_of_study?: string;
	    start_date?: string;
	    end_date?: string;
	
	    static createFrom(source: any = {}) {
	        return new EducationEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.degree = source["degree"];
	        this.school = source["school"];
	        this.field_of_study = source["field_of_study"];
	        this.start_date = source["start_date"];
	        this.end_date = source["end_date"];
	    }
	}
	export class ResumeMetadata {
	    filename?: string;
	    filetype?: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new ResumeMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.filetype = source["filetype"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class Candidate {
	    id: string;
	    name: string;
	    firstname: string;
	    lastname: string;
	    headline?: string;
	    account?: Record<string, string>;
	    job?: Record<string, string>;
	    stage: string;
	    stage_kind: string;
	    disqualified: boolean;
	    disqualification_reason?: string;
	    hired_at?: string;
	    moved_to_offer_at?: string;
	    sourced: boolean;
	    profile_url: string;
	    address?: string;
	    phone?: string;
	    email?: string;
	    domain?: string;
	    outlet?: string;
	    common_source?: string;
	    common_source_category?: string;
	    created_at: string;
	    updated_at: string;
	    resume_metadata?: ResumeMetadata;
	    image_url?: string;
	    cover_letter?: string;
	    summary?: string;
	    education_entries?: EducationEntry[];
	    experience_entries?: ExperienceEntry[];
	    skills?: any[];
	    answers?: any[];
	    resume_url?: string;
	    social_profiles?: SocialProfile[];
	    disqualified_at?: string;
	    withdrew?: boolean;
	    location?: Location;
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.firstname = source["firstname"];
	        this.lastname = source["lastname"];
	        this.headline = source["headline"];
	        this.account = source["account"];
	        this.job = source["job"];
	        this.stage = source["stage"];
	        this.stage_kind = source["stage_kind"];
	        this.disqualified = source["disqualified"];
	        this.disqualification_reason = source["disqualification_reason"];
	        this.hired_at = source["hired_at"];
	        this.moved_to_offer_at = source["moved_to_offer_at"];
	        this.sourced = source["sourced"];
	        this.profile_url = source["profile_url"];
	        this.address = source["address"];
	        this.phone = source["phone"];
	        this.email = source["email"];
	        this.domain = source["domain"];
	        this.outlet = source["outlet"];
	        this.common_source = source["common_source"];
	        this.common_source_category = source["common_source_category"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.resume_metadata = this.convertValues(source["resume_metadata"], ResumeMetadata);
	        this.image_url = source["image_url"];
	        this.cover_letter = source["cover_letter"];
	        this.summary = source["summary"];
	        this.education_entries = this.convertValues(source["education_entries"], EducationEntry);
	        this.experience_entries = this.convertValues(source["experience_entries"], ExperienceEntry);
	        this.skills = source["skills"];
	        this.answers = source["answers"];
	        this.resume_url = source["resume_url"];
	        this.social_profiles = this.convertValues(source["social_profiles"], SocialProfile);
	        this.disqualified_at = source["disqualified_at"];
	        this.withdrew = source["withdrew"];
	        this.location = this.convertValues(source["location"], Location);
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
	export class Comment {
	    id: string;
	    body: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Comment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.body = source["body"];
	        this.created_at = source["created_at"];
	    }
	}
	export class Conference {
	    type?: string;
	    url?: string;
	
	    static createFrom(source: any = {}) {
	        return new Conference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.url = source["url"];
	    }
	}
	export class DepartmentNode {
	    id: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new DepartmentNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	
	export class EventMember {
	    id: string;
	    name: string;
	    type: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new EventMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.status = source["status"];
	    }
	}
	export class Event {
	    id: string;
	    title: string;
	    description?: string;
	    type: string;
	    starts_at: string;
	    ends_at: string;
	    cancelled: boolean;
	    job: Record<string, string>;
	    candidate: Record<string, string>;
	    members?: EventMember[];
	    conference?: Conference;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.starts_at = source["starts_at"];
	        this.ends_at = source["ends_at"];
	        this.cancelled = source["cancelled"];
	        this.job = source["job"];
	        this.candidate = source["candidate"];
	        this.members = this.convertValues(source["members"], EventMember);
	        this.conference = this.convertValues(source["conference"], Conference);
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
	export class Salary {
	    salary_currency?: string;
	    min_value?: number;
	    max_value?: number;
	    salary_per?: string;
	
	    static createFrom(source: any = {}) {
	        return new Salary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.salary_currency = source["salary_currency"];
	        this.min_value = source["min_value"];
	        this.max_value = source["max_value"];
	        this.salary_per = source["salary_per"];
	    }
	}
	export class Job {
	    id: string;
	    title: string;
	    full_title: string;
	    shortcode: string;
	    code?: string;
	    state: string;
	    sample: boolean;
	    confidential: boolean;
	    department?: string;
	    department_hierarchy?: DepartmentNode[];
	    url: string;
	    application_url: string;
	    shortlink: string;
	    workplace_type: string;
	    location: Location;
	    locations?: Location[];
	    salary: Salary;
	    created_at: string;
	    updated_at: string;
	    keywords?: string[];
	    full_description?: string;
	    description?: string;
	    requirements?: string;
	    benefits?: string;
	    employment_type?: string;
	    industry?: string;
	    function?: string;
	    experience?: string;
	    education?: string;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.full_title = source["full_title"];
	        this.shortcode = source["shortcode"];
	        this.code = source["code"];
	        this.state = source["state"];
	        this.sample = source["sample"];
	        this.confidential = source["confidential"];
	        this.department = source["department"];
	        this.department_hierarchy = this.convertValues(source["department_hierarchy"], DepartmentNode);
	        this.url = source["url"];
	        this.application_url = source["application_url"];
	        this.shortlink = source["shortlink"];
	        this.workplace_type = source["workplace_type"];
	        this.location = this.convertValues(source["location"], Location);
	        this.locations = this.convertValues(source["locations"], Location);
	        this.salary = this.convertValues(source["salary"], Salary);
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.keywords = source["keywords"];
	        this.full_description = source["full_description"];
	        this.description = source["description"];
	        this.requirements = source["requirements"];
	        this.benefits = source["benefits"];
	        this.employment_type = source["employment_type"];
	        this.industry = source["industry"];
	        this.function = source["function"];
	        this.experience = source["experience"];
	        this.education = source["education"];
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
	export class EventFindResult {
	    event?: Event;
	    job?: Job;
	    candidate?: Candidate;
	
	    static createFrom(source: any = {}) {
	        return new EventFindResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.event = this.convertValues(source["event"], Event);
	        this.job = this.convertValues(source["job"], Job);
	        this.candidate = this.convertValues(source["candidate"], Candidate);
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
	
	
	
	
	export class Member {
	    id: string;
	    name: string;
	    email: string;
	    role: string;
	    headline?: string;
	    type?: string;
	    hris_role?: string;
	    roles?: string[];
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Member(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.email = source["email"];
	        this.role = source["role"];
	        this.headline = source["headline"];
	        this.type = source["type"];
	        this.hris_role = source["hris_role"];
	        this.roles = source["roles"];
	        this.active = source["active"];
	    }
	}
	
	

}

