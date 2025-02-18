package tasks

import (
	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/ansible-semaphore/semaphore/db"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/util"
	"github.com/gorilla/context"
)

func AddTaskToPool(d db.Store, taskObj db.Task, userID *int, projectID int) (db.Task, error) {
	taskObj.Created = time.Now()
	taskObj.Status = taskWaitingStatus
	taskObj.UserID = userID
	taskObj.ProjectID = projectID

	newTask, err := d.CreateTask(taskObj)
	if err != nil {
		return db.Task{}, err
	}

	pool.register <- &task{
		store:     d,
		task:      newTask,
		projectID: projectID,
	}

	objType := taskTypeID
	desc := "Task ID " + strconv.Itoa(newTask.ID) + " queued for running"
	_, err = d.CreateEvent(db.Event{
		UserID:      userID,
		ProjectID:   &projectID,
		ObjectType:  &objType,
		ObjectID:    &newTask.ID,
		Description: &desc,
	})

	return newTask, err
}

// AddTask inserts a task into the database and returns a header or returns error
func AddTask(w http.ResponseWriter, r *http.Request) {
	project := context.Get(r, "project").(db.Project)
	user := context.Get(r, "user").(*db.User)

	var taskObj db.Task

	if !helpers.Bind(w, r, &taskObj) {
		return
	}

	newTask, err := AddTaskToPool(helpers.Store(r), taskObj, &user.ID, project.ID)

	//taskObj.Created = time.Now()
	//taskObj.Status = taskWaitingStatus
	//taskObj.UserID = &user.ID
	//taskObj.ProjectID = project.ID
	//
	//newTask, err := helpers.Store(r).CreateTask(taskObj)
	//if err != nil {
	//	util.LogErrorWithFields(err, log.Fields{"error": "Bad request. Cannot create new task"})
	//	w.WriteHeader(http.StatusBadRequest)
	//	return
	//}

	//pool.register <- &task{
	//	store:     helpers.Store(r),
	//	task:      newTask,
	//	projectID: project.ID,
	//}
	//
	//objType := taskTypeID
	//desc := "Task ID " + strconv.Itoa(newTask.ID) + " queued for running"
	//_, err = helpers.Store(r).CreateEvent(db.Event{
	//	UserID:      &user.ID,
	//	ProjectID:   &project.ID,
	//	ObjectType:  &objType,
	//	ObjectID:    &newTask.ID,
	//	Description: &desc,
	//})

	if err != nil {
		util.LogErrorWithFields(err, log.Fields{"error": "Cannot write new event to database"})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	helpers.WriteJSON(w, http.StatusCreated, newTask)
}

// GetTasksList returns a list of tasks for the current project in desc order to limit or error
func GetTasksList(w http.ResponseWriter, r *http.Request, limit uint64) {
	project := context.Get(r, "project").(db.Project)
	tpl := context.Get(r, "template")

	var err error
	var tasks []db.TaskWithTpl

	if tpl != nil {
		tasks, err = helpers.Store(r).GetTemplateTasks(project.ID, tpl.(db.Template).ID, db.RetrieveQueryParams{
			Count: int(limit),
		})
	} else {
		tasks, err = helpers.Store(r).GetProjectTasks(project.ID, db.RetrieveQueryParams{
			Count: int(limit),
		})
	}

	if err != nil {
		util.LogErrorWithFields(err, log.Fields{"error": "Bad request. Cannot get tasks list from database"})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, tasks)
}

// GetAllTasks returns all tasks for the current project
func GetAllTasks(w http.ResponseWriter, r *http.Request) {
	GetTasksList(w, r, 0)
}

// GetLastTasks returns the hundred most recent tasks
func GetLastTasks(w http.ResponseWriter, r *http.Request) {
	GetTasksList(w, r, 200)
}

// GetTask returns a task based on its id
func GetTask(w http.ResponseWriter, r *http.Request) {
	task := context.Get(r, taskTypeID).(db.Task)
	helpers.WriteJSON(w, http.StatusOK, task)
}

// GetTaskMiddleware is middleware that gets a task by id and sets the context to it or panics
func GetTaskMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := context.Get(r, "project").(db.Project)
		taskID, err := helpers.GetIntParam("task_id", w, r)

		if err != nil {
			util.LogErrorWithFields(err, log.Fields{"error": "Bad request. Cannot get task_id from request"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		task, err := helpers.Store(r).GetTask(project.ID, taskID)
		if err != nil {
			util.LogErrorWithFields(err, log.Fields{"error": "Bad request. Cannot get task from database"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		context.Set(r, taskTypeID, task)
		next.ServeHTTP(w, r)
	})
}

// GetTaskOutput returns the logged task output by id and writes it as json or returns error
func GetTaskOutput(w http.ResponseWriter, r *http.Request) {
	task := context.Get(r, taskTypeID).(db.Task)
	project := context.Get(r, "project").(db.Project)

	var output []db.TaskOutput
	output, err := helpers.Store(r).GetTaskOutputs(project.ID, task.ID)

	if err != nil {
		util.LogErrorWithFields(err, log.Fields{"error": "Bad request. Cannot get task output from database"})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, output)
}

func StopTask(w http.ResponseWriter, r *http.Request) {
	targetTask := context.Get(r, "task").(db.Task)
	project := context.Get(r, "project").(db.Project)

	activeTask := pool.getTask(targetTask.ID)

	if activeTask == nil { // task not active, but exists in database
		activeTask = &task{
			store:     helpers.Store(r),
			task:      targetTask,
			projectID: project.ID,
		}
		err := activeTask.populateDetails()
		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		activeTask.setStatus(taskStoppedStatus)

		activeTask.createTaskEvent()
	} else {
		if activeTask.task.Status == taskRunningStatus {
			if activeTask.process == nil {
				panic("running process can not be nil")
			}

			if err := activeTask.process.Kill(); err != nil {
				helpers.WriteError(w, err)
			}
		}
		activeTask.setStatus(taskStoppingStatus)
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveTask removes a task from the database
func RemoveTask(w http.ResponseWriter, r *http.Request) {
	targetTask := context.Get(r, taskTypeID).(db.Task)
	editor := context.Get(r, "user").(*db.User)
	project := context.Get(r, "project").(db.Project)

	activeTask := pool.getTask(targetTask.ID)

	if activeTask != nil {
		// can't delete task in queue or running
		// task must be stopped firstly
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !editor.Admin {
		log.Warn(editor.Username + " is not permitted to delete task logs")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err := helpers.Store(r).DeleteTaskWithOutputs(project.ID, targetTask.ID)
	if err != nil {
		util.LogErrorWithFields(err, log.Fields{"error": "Bad request. Cannot delete task from database"})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
