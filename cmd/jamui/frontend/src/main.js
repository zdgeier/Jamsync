import './style.css';

// import logo from './assets/images/favicon.svg';
import {ListProjects, SelectDirectory} from '../wailsjs/go/main/App';

// document.getElementById('logo').src = logo;
let projectsElement = document.getElementById("projects");

try {
    ListProjects()
        .then((result) => {
            // Update result with data back from App.Greet()
            projectsElement.innerText = result;
        })
        .catch((err) => {
            console.error(err);
        });
} catch (err) {
    console.error(err);
}

let getDirButton = document.getElementById("opendirectory");
getDirButton.addEventListener('click', () => SelectDirectory())

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