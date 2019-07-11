import * as React from "react";
import { Course, Enrollment } from "../../../proto/ag_pb";
import { Search } from "../../components";
import { CourseManager, ILink, NavigationManager } from "../../managers";
import { IUserRelation } from "../../models";
import { ActionType, UserView } from "./UserView";

interface IUserViewerProps {
    navMan: NavigationManager;
    courseMan: CourseManager;
    acceptedUsers: IUserRelation[];
    pendingUsers: IUserRelation[];
    rejectedUsers: IUserRelation[];
    course: Course;
}

interface IUserViewerState {
    acceptedUsers: IUserRelation[];
    pendingUsers: IUserRelation[];
    rejectedUsers: IUserRelation[];
}

export class MemberView extends React.Component<IUserViewerProps, IUserViewerState, {}> {

    constructor(props: IUserViewerProps) {
        super(props);
        this.state = {
            acceptedUsers: this.props.acceptedUsers,
            pendingUsers: this.props.pendingUsers,
            rejectedUsers: this.props.rejectedUsers,
        };
    }
    public render() {
        const pendingActions = [
            { name: "Accept", uri: "accept", extra: "primary" },
            { name: "Reject", uri: "reject", extra: "danger" },
        ];
        return <div>
            <h1>{this.props.course.getName()}</h1>
            <Search className="input-group"
                    placeholder="Search for users"
                    onChange={(query) => this.handleSearch(query)}
                />
            {this.renderUserView()}
            {this.renderPendingView(pendingActions)}
            {this.renderRejectedView()}
        </div>;
    }

    public renderRejectedView() {
        if (this.state.rejectedUsers.length > 0) {
            return this.renderUsers(
                "Rejected users",
                this.state.rejectedUsers,
                [],
                ActionType.Menu,
                (user: IUserRelation) => {
                    const links = [];
                    if (user.link.state === Enrollment.UserStatus.REJECTED) {
                        links.push({ name: "Set pending", uri: "remove", extra: "primary" });
                    }
                    return links;
                });
        }
    }

    public renderUserView() {
        return this.renderUsers(
            "Registered users",
            this.state.acceptedUsers,
            [],
            ActionType.Menu,
            (user: IUserRelation) => {
                const links = [];
                if (user.link.state === Enrollment.UserStatus.TEACHER) {
                    links.push({ name: "This is a teacher", extra: "primary" });
                } else {
                    links.push({ name: "Make Teacher", uri: "teacher", extra: "primary" });
                    links.push({ name: "Reject", uri: "reject", extra: "danger" });
                }

                return links;
            });
    }

    public renderPendingView(pendingActions: ILink[]) {
        if (this.state.pendingUsers.length > 0) {
            return this.renderUsers("Pending users", this.state.pendingUsers, pendingActions, ActionType.InRow);
        }
    }

    public renderUsers(
        title: string,
        users: IUserRelation[],
        actions?: ILink[],
        linkType?: ActionType,
        optionalActions?: ((user: IUserRelation) => ILink[])) {
        console.log("Rendering users: " + title);
        return <div>
            <h3>{title}</h3>
            <UserView
                users={users}
                actions={actions}
                optionalActions={optionalActions}
                linkType={linkType}
                actionClick={(user, link) => this.handleAction(user, link)}
            >
            </UserView>
        </div>;
    }

    public handleAction(userRel: IUserRelation, link: ILink) {
        switch (link.uri) {
            case "accept":
                this.props.courseMan.changeUserState(userRel.link, Enrollment.UserStatus.STUDENT);
                break;
            case "reject":
                this.props.courseMan.changeUserState(userRel.link, Enrollment.UserStatus.REJECTED);
                break;
            case "teacher":
                if (confirm(
                    `Warning! This action is irreversible!
                    Do you want to continue assigning:
                    ${userRel.user.getName()} as a teacher?`,
                )) {
                    this.props.courseMan.changeUserState(userRel.link, Enrollment.UserStatus.TEACHER);
                }
                break;
            case "remove":
                this.props.courseMan.changeUserState(userRel.link, Enrollment.UserStatus.PENDING);
                break;
        }
        this.props.navMan.refresh();
    }

    private handleSearch(query: string): void {
        query = query.toLowerCase();
        const filteredAccepted: IUserRelation[] = [];
        const filteredPending: IUserRelation[] = [];
        const filteredRejected: IUserRelation[] = [];

        // we filter out every student group separately to ensure that student status is easily visible to teacher
        // filter accepted students
        this.props.acceptedUsers.forEach((user) => {
            if (user.user.getName().toLowerCase().indexOf(query) !== -1
                || user.user.getStudentid().toLowerCase().indexOf(query) !== -1
            ) {
                filteredAccepted.push(user);
            }
        });

        this.setState({
            acceptedUsers: filteredAccepted,
        });

        // filter pending students
        this.props.pendingUsers.forEach((user) => {
            if (user.user.getName().toLowerCase().indexOf(query) !== -1
                || user.user.getStudentid().toLowerCase().indexOf(query) !== -1
            ) {
                filteredPending.push(user);
            }
        });

        this.setState({
            pendingUsers: filteredPending,
        });

        // filter rejected students
        this.props.rejectedUsers.forEach((user) => {
            if (user.user.getName().toLowerCase().indexOf(query) !== -1
                || user.user.getStudentid().toLowerCase().indexOf(query) !== -1
            ) {
                filteredRejected.push(user);
            }
        });

        this.setState({
            rejectedUsers: filteredRejected,
        });

    }
}

export { UserView };
