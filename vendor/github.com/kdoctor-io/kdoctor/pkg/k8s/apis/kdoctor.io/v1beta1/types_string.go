// Copyright 2023 Authors of kdoctor-io
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"
	"strings"

	stringutil "github.com/kdoctor-io/kdoctor/pkg/utils/string"
)

func (in *TaskStatus) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&TaskStatus{`,
		`ExpectedRound:` + stringutil.ValueToStringGenerated(in.ExpectedRound),
		`DoneRound:` + stringutil.ValueToStringGenerated(in.DoneRound),
		`Finish:` + fmt.Sprintf("%v", in.Finish),
		`FinishTime:` + stringutil.ValueToStringGenerated(in.FinishTime),
		`LastRoundStatus:` + fmt.Sprintf("%+v", in.LastRoundStatus),
		`History:` + fmt.Sprintf("%+v", in.History),
		`Resource:` + in.Resource.String(),
		`}`,
	}, "")

	return s
}

func (in *TaskResource) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&TaskResource{`,
		`RuntimeName:` + fmt.Sprintf("%+v", in.RuntimeName),
		`RuntimeType:` + fmt.Sprintf("%+v", in.RuntimeType),
		`ServiceNameV4:` + stringutil.ValueToStringGenerated(in.ServiceNameV4),
		`ServiceNameV6:` + stringutil.ValueToStringGenerated(in.ServiceNameV6),
		`RuntimeStatus:` + fmt.Sprintf("%+v", in.RuntimeStatus),
		`}`,
	}, "")

	return s
}
