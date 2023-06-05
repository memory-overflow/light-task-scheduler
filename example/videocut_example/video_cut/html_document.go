package videocut

const htmlFile = `
<!DOCTYPE html>

<html>
		<head>
				<meta http-equiv=Content-Type content="text/html; charset=utf-8">
				<title>视频剪切</title>
				<script src="https://ajax.googleapis.com/ajax/libs/jquery/3.6.3/jquery.min.js"></script>
				<style>
						li.active {
								background-color: rgba(255,142,0,1.00);
						}
						body {
								font: normal 13px Verdana, Arial, sans-serif;
						}
				</style>
		</head>
		<body>
				<script src="html.js" type="text/javascript">
				</script>
		</body>
</html>
`

const jsFile = `
var ip = "127.0.0.1", curPath = ""
var curTaskPageSize = 10, curTaskPageNumber = 1
var curMediaPageSize = 10, curMediaPageNumber = 1

jQuery["postJSON"] = function( url, data, callback ) {
    // shift arguments if data argument was omitted
    if ( jQuery.isFunction( data ) ) {
        callback = data;
        data = undefined;
    }
    return jQuery.ajax({
        url: url,
        type: "POST",
        contentType:"application/json; charset=utf-8",
        dataType: "json",
        data: JSON.stringify(data),
        success: callback
    });
};

$(document).ready( function(){
    if (window.location.host != "") {
        ip = window.location.host
    }
    curPath = "task"
    drawBar()
    flashList()
    setInterval(flashList, 5000)
})


function createTask() {
    $("#mediainfo-table").remove()
    if (document.getElementById("taskinfo-table") != null) {
        return
    }
    var tb = $("<table>", {
        "id": "taskinfo-table",
    }).css({
        "float": "left",
        "list-style": "none",
        "padding": 0,
        "margin": "20px 20px",
    })
    var tbody = $("<tbody>")
    inputvideotr = $("<tr>")
    inputvideotr.append($("<td>", {
        "text": "待裁剪视频",
    })).append($("<td>").append($("<textarea>", {
            "id": "taskinfo-inputvideo",
            "height": "40px",
        }).css("resize", "none")
    ))
    starttr = $("<tr>")
    starttr.append($("<td>", {
        "text": "开始时间(s)",
    })).append($("<td>").append($("<textarea>", {
            "id": "taskinfo-start",
            "height": "15px",
        }).css("resize", "none")
    ))
    endtr = $("<tr>")
    endtr.append($("<td>", {
        "text": "结束时间(s)",
    })).append($("<td>").append($("<textarea>", {
            "id": "taskinfo-end",
            "height": "15px",
        }).css("resize", "none")
    ))
    confirmtr = $("<tr>")
    confirmtr.append($("<td>").append($("<button>", {
        "text": "确定",
    }).click({}, function(e) {
        body = {
            "InputVideo": $("#taskinfo-inputvideo").val(),
            "CutStartTime": parseFloat($("#taskinfo-start").val()),
            "CutEndTime": parseFloat($("#taskinfo-end").val()),
        }
        $.postJSON("http://" + ip + "/CreateTask", body, function(data, status) {
            if (data.ErrorCode != 0) {
                alert("create error: " + data.ErrorMessage)
                return
            }
            curPath = "task"
            $("#taskinfo-table").remove()
            $("li").removeClass("active");
            $("#task-list").addClass('active')
            flashList()
        })
    })))
    confirmtr.append($("<td>").append($("<button>", {
        "text": "取消",
    }).click({}, function(e) {
        $("#taskinfo-table").remove()
    })))
    tbody.append(inputvideotr)
    tbody.append(starttr)
    tbody.append(endtr)
    tbody.append(confirmtr)
    tb.append(tbody)
    tb.appendTo("ul")
}

function drawBar() {
    ul = $('<ul>', {'id': 'bar'})
    ul.append($('<li>', {
        'text': "任务列表",
        'class': "active",
        "id": "task-list",
    }).css({
        "float": "left",
        "list-style": "none",
        "padding": 0,
        "margin": "20px 20px",
    }).click({}, function(e) {
        $("li").removeClass("active");
        $(this).addClass('active')
        curPath = "task"
        flashList()
    }))
    // 添加上传按钮
    taskupload = $("<button>", {
        "text": "创建任务",
        "id": "upload-task",
    }).css({
        "float": "left",
        "list-style": "none",
        "padding": 0,
        "margin": "17px 20px",
    }).click({}, function(e) {
        createTask()
    })
    ul.appendTo("body")
    taskupload.appendTo("body")
}

function deleteTask(data) {
    body = {
        "TaskId": data.taskId,
    }
    $.postJSON("http://" + ip + "/DeleteTask", body, function(data, status) {
        if (data.ErrorCode != 0) {
            alert("deleteTask error: " + data.ErrorMessage)
            return
        }
        // 操作后要刷新列表
        flashList()
    })
}

function operatorTask(data) {
    acation = ""
    if (data.status == "等待执行" || data.status == "执行中") {
        action = "StopTask"
    } else {
        action = "StartTask"
    }
    body = {
        "TaskID": data.taskId,
    }
    $.postJSON("http://" + ip + "/" + action, body, function(data, status) {
        if (data.ErrorCode != 0) {
            alert(action + " error: " + data.ErrorMessage)
            return
        }
        // 操作后要刷新列表
        flashList()
    })
}


function showResult(data) {
    curPath = "result"
    $("#task").remove()
    $("#result").remove()
    $("li").removeClass("active")

    result = $('<div>', {'id': 'result'}).css("clear", "both")
    result.append("<p>", "任务" + data.task.TaskId + "裁剪前视频</p>")
    video1 = $('<video>', {
        "src": "http://" + ip + "/Download?path=" + data.task["InputVideo"],
        "type": "video/mp4",
        "controls": "true"
    })
    result.append(video1)

    result.append("<p>", "任务" + data.task.TaskId + "裁剪后视频</p>")
    video2 = $('<video>', {
        "src": "http://" + ip + "/Download?path=" + data.task["OutputVideo"],
        "type": "video/mp4",
        "controls": "true"
    })

    
    result.append(video2)
    result.appendTo("body")
}

function showTaskList() {
    $.postJSON("http://" + ip + "/TaskList", {
        "PageNumber": curTaskPageNumber,
        "PageSize": curTaskPageSize,
    }, function(data, status) {
        if (data.ErrorCode != 0) {
            alert("TaskList error: " + data.ErrorMessage)
            return
        }
        var $table = $("<table>", {
            'border':'1',
            'width': '100%',
            'id': "task"
        })
        var $thead = $("<thead>")
        var $tbody = $("<tbody>")
        var headers = ["TaskId", "InputVideo", "裁剪时间段", "OutputVideo", "Status", "StartTime", "EndTime", "FailedReason"]
        for (var i = 0; i < headers.length; i++) {
            $thead.append($('<th>', {'text': headers[i]}))
        }
        $thead.append($('<th>', {'text': "操作"}))

        // add table
        var l = 0
        if (data.Tasks != null) {
            l = data.Tasks.length
        }
        for(var i = 0; i < l; i++) {
            newTr = $('<tr>')
            for (var j = 0; j < headers.length; j++) {
              if (headers[j] != "裁剪时间段") {
                newTr.append($('<td>', {
                  'text': data.Tasks[i][headers[j]],
                  'white-space': 'nowrap',
                }))
              } else {
                newTr.append($('<td>', {
                  'text': data.Tasks[i].CutStartTime.toFixed(1) + "s - " + data.Tasks[i].CutEndTime.toFixed(1) + "s",
                  'white-space': 'nowrap',
                }))
              }
            }
            var name = "执行任务"
            if (data.Tasks[i].Status == "执行中" || data.Tasks[i].Status  == "等待执行") {
                name = "停止任务"
            }
            // add operator button
            var taskId = data.Tasks[i].TaskId
            td = $('<td>')
            td.append($('<button>', {
                'text': name,
                'id': "button" + taskId,
            }).click({
                taskId: taskId,
                status: data.Tasks[i].Status
            }, function(e) {
                operatorTask(e.data)
            }))
            td.append($('<button>', {'text': "删除任务"}).click({taskId: taskId}, function(e) {
                deleteTask(e.data)
            }))
            td.append($('<button>', {
                'text': "查看结果",
                'disabled': (data.Tasks[i].Status != "执行成功"),
            }).click({task: data.Tasks[i]}, function(e) {
                showResult(e.data)
            }))
            newTr.append(td)
            $tbody.append(newTr)
        }
        // add turn page
        newTr = $('<tr>')
        td = $("<td>", {
            "text": "当前第" + curTaskPageNumber + "页",
            "id": "pagenumber"
        })
        newTr.append(td)
        td = $("<td>").append($("<button>", {
            "text": "上一页",
        }).click({}, function(e) {
            if (curTaskPageNumber > 1) {
                curTaskPageNumber = curTaskPageNumber - 1
            }
            flashList()
        }))
        newTr.append(td)
        td = $("<td>").append($("<button>", {
            "text": "下一页",
        }).click({}, function(e) {
            curTaskPageNumber = curTaskPageNumber + 1
            flashList()
        }))
        newTr.append(td)
        $tbody.append(newTr)
        // Add table to DOM
        $table.append($thead);
        $table.append($tbody);

        $("#task").remove();
        $table.appendTo("body");
    });
}

function flashList() {
    if (curPath == "task") {
        $("#result").remove()
        showTaskList()
    }
}
`
