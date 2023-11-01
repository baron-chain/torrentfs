// Copyright 2023 The CortexTheseus Authors
// This file is part of the CortexTheseus library.
//
// The CortexTheseus library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The CortexTheseus library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the CortexTheseus library. If not, see <http://www.gnu.org/licenses/>

package job

type Job struct {
	id       uint64
	status   int
	category int
	ref      string
}

func New() *Job {
	job := new(Job)
	return job
}

func (j *Job) ID() uint64 {
	return j.id
}

func (j *Job) Category() int {
	return j.category
}

func (j *Job) Status() int {
	return j.status
}

func (j *Job) End() {
}
