import './style.css';

import logo from './assets/images/favicon.svg';
import { ChangeDirectory, SelectDirectory, ProjectExists, GetInfo, StateFileExists, InitExistingProject, InitNewProject} from '../wailsjs/go/main/App';

document.getElementById('logo').src = logo;
// let projectsElement = document.getElementById("projects");

// try {
//     ListProjects()
//         .then((result) => {
//             // Update result with data back from App.Greet()
//             projectsElement.innerText = result;
//         })
//         .catch((err) => {
//             console.error(err);
//         });
// } catch (err) {
//     console.error(err);
// }

document.getElementById("screen-open-directory-initializeNewProject").addEventListener('click', async () => {
    let screenOpenDirectoryEl = document.getElementById("screen-open-directory");
    screenOpenDirectoryEl.classList.add("is-hidden");
    let screenInitNewProjectEl = document.getElementById("screen-init-new-project");
    screenInitNewProjectEl.classList.remove("is-hidden");
    // if (initNewProjectInput.value === "") {
    //     statusEl.innerHTML = "input a project name"
    //     return
    // }
    // const path = await SelectDirectory()
    // statusEl.innerHTML = "";
    // console.log(path, await StateFileExists(path))
    // if (await StateFileExists(path)) {
    //     statusEl.innerHTML = "already initialized";
    // } else {
    //     console.log(initNewProjectInput.value)
    //     InitializeNewProject(path, initNewProjectInput.value);
    // }
});

document.getElementById("screen-init-new-project-submit").addEventListener('click', async() => {
    const projectName = document.getElementById("screen-init-new-project-name").value;
    const path = document.getElementById("screen-init-new-project-path").value;
    if (projectName === "" || path === "") {
        document.getElementById("screen-init-new-project-status").innerHTML = "Project name and path must not be empty."
        return
    }
    if (await StateFileExists(path)) {
        document.getElementById("screen-init-new-project-status").innerHTML = ".jamsync file already exists in directory."
        return
    }
    
    if (await ProjectExists(projectName)) {
        console.log("initializing existing project ", projectName, "at", path)
        await InitExistingProject(path, projectName)
    } else {
        console.log("initializing new project ", projectName, "at", path)
        await InitNewProject(path, projectName)
    }

    document.getElementById("screen-init-new-project").classList.add("is-hidden");
    document.getElementById("screen-project-status").classList.remove("is-hidden");

    window.setInterval(async () => {
        const newStatus = await GetInfo();
        if (JSON.stringify(newStatus) !== JSON.stringify(currentStatus)) {
            document.getElementById("screen-project-status-info").innerHTML = "";
            for (const line of newStatus) {
                let temp = document.createElement("li");
                temp.innerHTML = line;
                document.getElementById("screen-project-status-info").appendChild(temp);
            }
            currentStatus = newStatus;
        }
    }, 2000);
});

document.getElementById("screen-init-new-project-path-dialog").addEventListener('click', async () => {
    const path = await SelectDirectory()
    document.getElementById("screen-init-new-project-path").value = path;
    document.getElementById("screen-init-new-project-status").innerHTML = "";
});


let currentStatus = [];
document.getElementById("screen-open-directory-openExistingProject").addEventListener('click', async () => {
    const path = await SelectDirectory()
    document.getElementById("screen-init-new-project-path").value = path;
    if (path === "") return;

    await ChangeDirectory(path)

    if (!(await StateFileExists(path))) {
        document.getElementById("screen-open-directory-status").innerHTML = ".jamsync file not found at selected path. Use \"Initialize Project\" to initialize the directory.";
        return
    }

    document.getElementById("screen-open-directory").classList.add("is-hidden");
    document.getElementById("screen-project-status").classList.remove("is-hidden");

    window.setInterval(async () => {
        const newStatus = await GetInfo();
        if (JSON.stringify(newStatus) !== JSON.stringify(currentStatus)) {
            document.getElementById("screen-project-status-info").innerHTML = "";
            for (const line of newStatus) {
                let temp = document.createElement("li");
                temp.innerHTML = line;
                document.getElementById("screen-project-status-info").appendChild(temp);
            }
            currentStatus = newStatus;
        }
    }, 2000);
});





// let initNewProjectInput = document.getElementById("initializeNewProjectInput");
// 
// let openExistingProjectButton = document.getElementById("openExistingProject");
// openExistingProjectButton.addEventListener('click', async () => {
// 
// })

// window.listprojects = function() {
//     try {
//         ListProjects()
//             .then((result) => {
//                 // Update result with data back from App.Greet()
//                 projectsElement.innerText = result;
//             })
//             .catch((err) => {
//                 console.error(err);
//             });
//     } catch (err) {
//         console.error(err);
//     }
// }
// 