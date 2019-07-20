package web

import (
	"context"
	"fmt"
	"log"

	pb "github.com/autograde/aguis/ag"
	"github.com/autograde/aguis/scm"
)

// getCourses returns all courses.
func (s *AutograderService) getCourses() (*pb.Courses, error) {
	courses, err := s.db.GetCourses()
	if err != nil {
		return nil, err
	}
	return &pb.Courses{Courses: courses}, nil
}

// getCoursesWithEnrollment returns all courses that match the provided enrollment status.
func (s *AutograderService) getCoursesWithEnrollment(request *pb.RecordRequest) (*pb.Courses, error) {
	courses, err := s.db.GetCoursesByUser(request.ID, request.Statuses...)
	if err != nil {
		return nil, err
	}
	return &pb.Courses{Courses: courses}, nil
}

// createEnrollment enrolls a user in a course.
func (s *AutograderService) createEnrollment(request *pb.Enrollment) error {
	enrollment := pb.Enrollment{
		UserID:   request.GetUserID(),
		CourseID: request.GetCourseID(),
		Status:   pb.Enrollment_PENDING,
	}
	return s.db.CreateEnrollment(&enrollment)
}

// updateEnrollment accepts or rejects a user to enroll in a course.
func (s *AutograderService) updateEnrollment(ctx context.Context, sc scm.SCM, request *pb.Enrollment) error {
	enrollment, err := s.db.GetEnrollmentByCourseAndUser(request.CourseID, request.UserID)
	if err != nil {
		return err
	}

	switch request.Status {
	case pb.Enrollment_REJECTED:
		return s.db.RejectEnrollment(request.UserID, request.CourseID)

	case pb.Enrollment_PENDING:
		return s.db.SetPendingEnrollment(request.UserID, request.CourseID)

	case pb.Enrollment_STUDENT:
		course, student := enrollment.GetCourse(), enrollment.GetUser()

		// check whether user repo already exists,
		// which could happen if accepting a previously rejected student
		userRepoQuery := &pb.Repository{
			OrganizationID: course.GetOrganizationID(),
			UserID:         request.GetUserID(),
			RepoType:       pb.Repository_USER,
		}
		repos, err := s.db.GetRepositories(userRepoQuery)
		if err != nil {
			return err
		}
		if len(repos) > 0 {
			// repo already exist, update enrollment in database
			return s.db.EnrollStudent(request.UserID, request.CourseID)
		}

		// create user repo, user team, and add user to students team
		repo, err := updateReposAndTeams(ctx, sc, course, student.GetLogin(), request.GetStatus())
		if err != nil {
			s.logger.Errorf("UpdateEnrollment: failed to update repos or team membersip for student %s: %s", student.Login, err.Error())
			return err
		}

		// add student repo to database if SCM interaction above was successful
		dbRepo := pb.Repository{
			OrganizationID: course.GetOrganizationID(),
			UserID:         request.GetUserID(),
			RepoType:       pb.Repository_USER,
			RepositoryID:   repo.ID,
			HTMLURL:        repo.WebURL,
		}
		if err := s.db.CreateRepository(&dbRepo); err != nil {
			return err
		}
		return s.db.EnrollStudent(request.UserID, request.CourseID)

	case pb.Enrollment_TEACHER:
		course, teacher := enrollment.GetCourse(), enrollment.GetUser()

		// make owner, remove from students, add to teachers
		if _, err := updateReposAndTeams(ctx, sc, course, teacher.GetLogin(), request.GetStatus()); err != nil {
			s.logger.Errorf("UpdateEnrollment: failed to update team membersip for teacher %s: %s", teacher.Login, err.Error())
			return err
		}
		return s.db.EnrollTeacher(teacher.ID, course.ID)
	}

	return fmt.Errorf("unknown enrollment")
}

func updateReposAndTeams(ctx context.Context, sc scm.SCM, course *pb.Course, login string, state pb.Enrollment_UserStatus) (*scm.Repository, error) {
	org, err := sc.GetOrganization(ctx, course.OrganizationID)
	if err != nil {
		return nil, err
	}

	// options to use when updating team membership
	teamOpt := &scm.TeamMembershipOptions{
		Organization: org,
		TeamSlug:     "students",
		TeamID:       0,
		Username:     login,
	}

	switch state {
	case pb.Enrollment_STUDENT:
		// add student to the organization's "students" team
		if err := sc.AddTeamMember(ctx, teamOpt); err != nil {
			return nil, err
		}
		// create user repo and personal team for the student
		repo, _, err := createRepoAndTeam(ctx, sc, course, pb.StudentRepoName(login), login, []string{login})
		if err != nil {
			return nil, err
		}
		return repo, nil

	case pb.Enrollment_TEACHER:
		// if teacher, promote to owner, remove from students team, add to teachers team
		orgUpdate := &scm.OrgMembership{
			Organization: org,
			Username:     login,
			Role:         "admin",
		}
		// when promoting to teacher, promote to organization owner as well
		if err = sc.UpdateOrgMembership(ctx, orgUpdate); err != nil {
			log.Println("UpdateRepoAndTeam: failed to update org membership: ", err.Error())
			return nil, err
		}
		// then remove from students team
		if err = sc.RemoveTeamMember(ctx, teamOpt); err != nil {
			log.Println("UpdateRepoAndTeam: failed to remove team member: ", err.Error())
			return nil, err
		}
		// and add to teachers team
		teamOpt.TeamSlug = "teachers"
		teamOpt.Role = "maintainer"
		return nil, sc.AddTeamMember(ctx, teamOpt)
	}
	return nil, fmt.Errorf("unknown enrollment")
}

// GetCourse returns a course object for the given course id.
func (s *AutograderService) getCourse(courseID uint64) (*pb.Course, error) {
	return s.db.GetCourse(courseID)
}

// getSubmission returns the submission of the current user for the given assignment.
func (s *AutograderService) getSubmission(currentUser *pb.User, request *pb.RecordRequest) (*pb.Submission, error) {
	// ensure that the submission belongs to the current user
	query := &pb.Submission{AssignmentID: request.ID, UserID: currentUser.ID}
	return s.db.GetSubmission(query)
}

// getSubmissions returns all the latests submissions for a user of the given course.
func (s *AutograderService) getSubmissions(request *pb.SubmissionRequest) (*pb.Submissions, error) {
	// only one of user ID and group ID will be set; enforced by IsValid on pb.SubmissionRequest
	query := &pb.Submission{
		UserID:  request.GetUserID(),
		GroupID: request.GetGroupID(),
	}
	submissions, err := s.db.GetSubmissions(request.GetCourseID(), query)
	if err != nil {
		return nil, err
	}
	return &pb.Submissions{Submissions: submissions}, nil
}

// approveSubmission approves the given submission.
func (s *AutograderService) approveSubmission(submissionID uint64) error {
	return s.db.UpdateSubmission(submissionID, true)
}

// updateCourse updates an existing course.
func (s *AutograderService) updateCourse(ctx context.Context, sc scm.SCM, request *pb.Course) error {
	// ensure the course exists
	_, err := s.db.GetCourse(request.ID)
	if err != nil {
		return err
	}
	// ensure the organization exists
	_, err = sc.GetOrganization(ctx, request.OrganizationID)
	if err != nil {
		return err
	}
	return s.db.UpdateCourse(request)
}

func (s *AutograderService) getEnrollment(request *pb.EnrollmentRequest) (*pb.Enrollment, error) {
	return s.db.GetEnrollmentByCourseAndUser(request.GetCourseID(), request.GetUserID())

}

// getEnrollmentsByCourse get all enrollments for a course that match the given enrollment request.
func (s *AutograderService) getEnrollmentsByCourse(request *pb.EnrollmentsRequest) (*pb.Enrollments, error) {
	enrollments, err := s.db.GetEnrollmentsByCourse(request.CourseID, request.States...)
	if err != nil {
		return nil, err
	}

	// to populate response only with users who are not member of any group, we must filter the result
	if request.FilterOutGroupMembers {
		enrollmentsWithoutGroups := make([]*pb.Enrollment, 0)
		for _, enrollment := range enrollments {
			if enrollment.GroupID == 0 {
				enrollmentsWithoutGroups = append(enrollmentsWithoutGroups, enrollment)
			}
		}
		enrollments = enrollmentsWithoutGroups
	}
	return &pb.Enrollments{Enrollments: enrollments}, nil
}

// getRepositoryURL returns the repository information
func (s *AutograderService) getRepositoryURL(currentUser *pb.User, request *pb.RepositoryRequest) (*pb.URLResponse, error) {
	course, err := s.db.GetCourse(request.GetCourseID())
	if err != nil {
		return nil, err
	}
	userRepoQuery := &pb.Repository{
		OrganizationID: course.GetOrganizationID(),
		RepoType:       request.GetType(),
	}
	if request.Type == pb.Repository_USER {
		userRepoQuery.UserID = currentUser.GetID()
	}

	repos, err := s.db.GetRepositories(userRepoQuery)
	if err != nil {
		return nil, err
	}
	if len(repos) != 1 {
		return nil, fmt.Errorf("found %d repositories for query %+v", len(repos), userRepoQuery)
	}
	return &pb.URLResponse{URL: repos[0].HTMLURL}, nil
}
