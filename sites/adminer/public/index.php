<?php
// Sabdopalon wrapper: local-dev Adminer.
// - prefills the login form with the bundled MariaDB defaults
// - allows the empty root password (Adminer 5 refuses it by default; the
//   daemon only listens on 127.0.0.1, matching the Laragon/XAMPP posture)
function adminer_object() {
	return new class extends \Adminer\Adminer {
		function login($login, $password) {
			return true;
		}
	};
}
if (($_SERVER['REQUEST_METHOD'] ?? 'GET') === 'GET' && !isset($_GET['username'])) {
	$_GET['server'] = $_GET['server'] ?? '127.0.0.1';
	$_GET['username'] = 'root';
}
require __DIR__ . '/adminer.php';
