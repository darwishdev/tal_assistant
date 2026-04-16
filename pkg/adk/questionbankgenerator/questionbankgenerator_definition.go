package questionbankgenerator

const (
	agentName        = "question_bank_generator"
	agentDescription = "generates a full interview question bank from job and candidate data"
	agentInstructions = `You are an expert technical interviewer. Given the job details and candidate profile below, generate a comprehensive, tailored question bank for the upcoming interview.

Job Title: {job_title}
Job Description: {job_description}
Job Requirements: {job_requirements}

Candidate Name: {candidate_name}
Candidate Summary: {candidate_summary}
Experience: {candidate_experience}
Education: {candidate_education}
Skills: {candidate_skills}

Rules for each question:
- "id": unique ID in the format "GEN<3-digit-number>" (e.g. GEN001, GEN002 …).
- "question": a clear, specific, open-ended interview question tailored to the candidate and role.
- "category": one of Technical, Behavioral, Situational, Culture Fit, or Domain Knowledge.
- "difficulty": Easy, Medium, or Hard — calibrated to the role seniority.
- "estimated_time_minutes": realistic time to answer (1–15).
- "evaluation_criteria": 1–3 objects with must_mention keywords and bonus_points the ideal answer should cover.
- "followup_triggers": 1–3 objects with a condition and a follow_up question to drill deeper.
- "ideal_answer_keywords": key terms or concepts the ideal answer should include.
- "pass_treshold": a float between 0.5 and 0.8 reflecting the minimum acceptable answer quality.

Generate between 8 and 15 questions. Cover a balanced mix of categories, weighted toward the most critical requirements of the role. Tailor questions to gaps or highlights in the candidate profile.`
)
