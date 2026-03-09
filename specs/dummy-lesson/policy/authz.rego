package authz

import rego.v1

# @ownership course: courses.instructor_id
# @ownership lesson: courses.instructor_id via lessons.course_id
# @ownership review: reviews.user_id

default allow := false

# 강의 생성: 인증된 사용자 누구나
allow if {
    input.action == "create"
    input.resource == "course"
}

# 강의 수정/삭제/공개: 강의 소유자만
allow if {
    input.action in {"update", "delete", "publish"}
    input.resource == "course"
    input.user.id == input.resource_owner
}

# 레슨 생성/수정/삭제: 강의 소유자만
allow if {
    input.action in {"create", "update", "delete"}
    input.resource == "lesson"
    input.user.id == input.resource_owner
}

# 수강 등록: 인증된 사용자 누구나
allow if {
    input.action == "enroll"
    input.resource == "course"
}

# 리뷰 삭제: 리뷰 작성자만
allow if {
    input.action == "delete"
    input.resource == "review"
    input.user.id == input.resource_owner
}
