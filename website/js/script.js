//*******************************************************************************
//CONSTANTS
const MIN_PASSWORD_LENGTH = 8;
const BAD_PASSWORDS = [
	"password",
	"password1",
	"12345678",
	"123456789",
	"123123123",
	"00000000",
	"1234567890",
	"asdfasdf",
	"asdfghjkl",
	"testtest",
	"admin@example.com"
];

const MIN_CHARGE = 0.5;

//*******************************************************************************
//COMMON FUNCS

//REGEX MATCH EMAIL
//true if input is a valid email
function validateEmail(email) {
	var regex = /^(([^<>()[\]\\.,;:\s@\"]+(\.[^<>()[\]\\.,;:\s@\"]+)*)|(\".+\"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
	return regex.test(email);
}

//CHECK IF TWO PASSWORDS MATCH
function doWordsMatch (word1, word2) {
	if (word1 === word2) {
		return true;
	}

	return false;
}

//CHECK IF A PASSWORD IS LONG ENOUGH
function isLongPassword (password) {
	if (password.length < MIN_PASSWORD_LENGTH) {
		return false;
	}

	return true;
}

//CHECK IF A GIVEN PASSWORD IS IN THE LIST OF TOO EASY TO GUESS PASSWORDS
function isSimplePassword (password) {
	if (BAD_PASSWORDS.indexOf(password) !== -1) {
		return true;
	}

	return false;
}

//SHOW AN ERROR MESSAGE IN A FORM
//title is usually "error" or "warning"
//message is something descriptive about the error so the user can fix the problem
//parentElem is the parent html element where the <div .error-msg> is located
function showPanelMessage (msg, type, elem) {
	elem.html('<div class="alert alert-' + type +'">' + msg + '</div>');
	return;
}

//SHOW MESSAGES IN MODALS
function showModalMessage (msg, type, elem) {
	elem.html('<div class="alert alert-' + type + '">' + msg + '</div>');
	return
}

//*******************************************************************************
//FUNCS

//SHOW SPECIFIC ACTION PANEL WHEN NAV BUTTONS ARE CLICKED
$('body').on('click', '.action-btn', function() {
	//SPEED AT WHICH TO HIDE AND SHOW PANELS
	const PANEL_TRANSITION_SPEED = 'fast';

	//GET TARGET PANEL TO OPEN
	var dataAction = 	$(this).data("action");

	//NEW PANEL ELEMENT
	//the containing div for the entire panel
	var panelToShow = 	$('#' + dataAction);

	//CHECK IF TARGET PANEL IS ALREADY OPEN
	//if it is visible to the user
	//don't do anything if the correct panel is already visible
	if (panelToShow.hasClass('show')) {
		return;
	}

	//CURRENTLY VISIBLE PANEL
	//this is the panel that will be hidden
	var panelToHide = 	$('.action-panels.show');

	//TRANSITION
	//fade out the current panel
	//fade in the new panel
	//using callbacks so that the fade out is completed before the fade in occurs
	panelToHide.fadeOut(PANEL_TRANSITION_SPEED, function() {
		panelToHide.removeClass('show');

		panelToShow.fadeIn(PANEL_TRANSITION_SPEED, function() {
			panelToShow.addClass('show');
			return;
		});

		return;
	});

	//CLEAR ALL THE PANELS TO DEFAULT OPTIONS
	resetAddCardPanel();
	resetChargeCardPanel(true)
});

//VALIDATE THE INITIAL ADMIN CREATE FORM
//make sure the passwords are strong and match
$('#create-init-admin').submit(function (e) {
	//get passwords from inputs
	var pass1 = $('#password1').val();
	var pass2 = $('#password2').val();
	var msg = 	$('#create-init-admin .msg');

	//check if passwords match
	if (doWordsMatch(pass1, pass2) === false) {
		e.preventDefault();
		showPanelMessage("The passwords do not match.", 'danger', msg);
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(pass1) === false) {
		e.preventDefault();
		showPanelMessage("Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", 'danger', msg);
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(pass1) === true) {
		e.preventDefault();
		essage("The password you provided is too simple. Please choose a better password.", 'danger', msg);
		return false;
	}

	//passwords validated
	//form will submit and save admin user
	//user will see a success/error page
	//then user can log in on main login page
});

//ON PAGE LOAD
$(function () {
	//ENABLE TOOLTIPS
	$('[data-toggle="tooltip"]').tooltip();

	//SET AJAX DEFAULT DATA TYPE
	$.ajaxSetup({
		dataType: 'json'
	});

	return;
});

//GET LIST OF CARDS
//gets a list of cards by customer name and customer id
//this is used to build the datalist of autocompletion in remove, charge, and reports
//the datalist element is right after the header of the page (before the main body elements)
//the customer id is the appengine datastore id and is used to look up the full customer details when a charge is performed
function getCards() {
	var customerList = $('#customer-list');

	$.ajax({
		type: 	"GET",
		url: 	"/card/get/all/",
		beforeSend: function() {
			console.log("Loading cards...");
			customerList.html('<option value="Loading...">');
			return;
		},
		error: function (r) {
			console.log(r);
			console.log(JSON.parse(r['responseText']));
			customerList.html('<option value="Could Not Load">');
			return;
		},
		success: function (j) {
			console.log("Loading cards...done!")

			//put results in data list
			var data = j['data'];
			customerList.html('');
			
			//check if no cards exist
			if (data.length === 0) {
				customerList.html('<option value="None exist yet!" data-id="0">');
				return;
			}

			//list each card
			//store the datastore id for looking up data on just this one card
			data.forEach(function (elem, index) {
				var name = 	elem['customer_name'];
				var id = 	elem['id'];

				customerList.append('<option value="' + name + '" data-id="' + id + '">');
			});
			return;
		}
	});
}

//GET VALUE OF CARD SELECTED FROM INPUT AUTOCOMPLETE LIST
//gets the id from the data- attribute of the selected option in the datalist
//does not use value b/c value is the customer name
//in: autocompleteElement: an html input element that used a datalist
//out: the id of the chosen customer
function getCardIdFromDataList(autocompleteElement) {
	var selectedOptionValue = 	autocompleteElement.val();
	var options = 				$('#customer-list option');
	var id = 					"";

	options.each(function() {
		var elemValue = $(this).val();
		var elemId = 	$(this).data('id');

		if (selectedOptionValue === elemValue) {
			id = elemId;
			return false;
		}
	});

	return id;
}

//GENERATE LIST OF YEARS FOR CARD EXPIRATION
//displays a list of future years for credit card expirations
//done programmatically so a list of years does not have to be updated as years change
//done on page load (code is in the html file)
//fills in a <select> with <options>
function generateExpirationYears() {
	console.log("Loading expiration years...");

	//element to displays years in
	var elem = $('#card-exp-year');
	elem.html('');

	//get current year
	var d = 	new Date()
	var year = 	d.getFullYear();

	//default first value
	elem.append('<option value="0">Please choose.</option>');

	//options for years
	for (var i = year; i < year + 11; i ++) {
		elem.append('<option value=' + i + '>' + i + '</option>');
	}

	console.log('Loading expiration years...done!');
	return;
}

//GET LIST OF USERNAMES AND IDS
//fill in the drop downs for editing users and changing passwords
function getUsers() {
	//there are two of these selects (change pwd & update user)
	var userList = $('.user-list');

	$.ajax({
		type: 	"GET",
		url: 	"/users/get/all/",
		beforeSend: function() {
			userList.html('<option value="0">Loading...</option>').attr('disabled', true);
			return;
		},
		error: function (r) {
			userList.html('<option value="0">Error (please see dev tools)</option>');
			console.log(r);
			return;
		},
		success: function (r) {
			//clear options
			userList.html('');
			userList.append("<option value='0'>Please choose...</option>").attr('disabled', false);

			//display list of users in selects
			//do not show 'administrator' user in list so it cannot be updates
			var users = r['data'];
			users.forEach(function (u, index) {
				if (u['username'] === "administrator") {
					return;
				}

				userList.append('<option value="' + u['id'] + '">' + u['username'] + '</option>');
				return;
			});

			return;
		}
	});
}

//*******************************************************************************
//ADD USER

//ADD A NEW USER
//validate user data and save user via ajax call
$('#form-new-user').submit(function (e) {
	//gather inputs
	var username = 		$('#form-new-user .username').val();
	var password1 = 	$('#form-new-user .password1').val();
	var password2 = 	$('#form-new-user .password2').val();
	var addCards = 		$('#form-new-user .can-add-cards input:checked').val();
	var removeCards = 	$('#form-new-user .can-remove-cards input:checked').val();
	var chargeCards = 	$('#form-new-user .can-charge-cards input:checked').val();
	var reports = 		$('#form-new-user .can-view-reports input:checked').val();
	var admin = 		$('#form-new-user .is-admin input:checked').val();
	var active = 		$('#form-new-user .is-active input:checked').val();
	var msgElem = 		$('#form-new-user .msg');
	var submit = 		$('#form-new-user-submit');

	//check if username is an email
	if (validateEmail(username) === false) {
		e.preventDefault();
		showModalMessage('You must provide an email address as a username.', 'danger', msgElem);
		return false;
	}

	//validate password
	//check if passwords match
	if (doWordsMatch(password1, password2) === false) {
		e.preventDefault();
		showModalMessage('The passwords do not match.', 'danger', msgElem);
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(password1) === false) {
		e.preventDefault();
		showModalMessage('Your password is too short. It must be at least ' + MIN_PASSWORD_LENGTH + ' characters.', 'danger', msgElem);
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(password1) === true) {
		e.preventDefault();
		showModalMessage('Your password too simple. Choose a more complex password.', 'danger', msgElem);
		return false;
	}

	//clear any alerts
	msgElem.html('');

	//add user via ajax call
	e.preventDefault();
	$.ajax({
		type: 	'POST',
		url: 	'/users/add/',
		data: {
			username: 		username,
			password1: 		password1,
			password2: 		password2,
			addCards: 		addCards,
			removeCards: 	removeCards,
			chargeCards: 	chargeCards,
			reports: 		reports,
			admin: 			admin,
			active: 		active
		},
		beforeSend: function() {
			//disable add user btn and show working message
			submit.attr("disabled", true);
			showModalMessage("Saving user...", "info", msgElem);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showModalMessage(j['data']['error_msg'], 'danger', msgElem);
				console.log(j);
				return;
			}

			submit.attr("disabled", true);
			return;
		},
		success: function (r) {
			showModalMessage("New user was saved sucessfully!", "success", msgElem);

			//reset inputs
			//hide success message
			setTimeout(function() {
				submit.attr("disabled", false);
				resetAddUserModal();
			}, 3000);
		}
	});
	return false;
});

//RESET NEW USER MODAL TO DEFAULT VALUES
function resetAddUserModal() {
	$('#form-new-user .username, #form-new-user .password1, #form-new-user .password2').val('');
	$('#form-new-user .default').attr("checked", true).parent('label').addClass('active').siblings('label').removeClass('active')
	$('.msg').html('');
	return;
}

//RESET ADD NEW USER MODAL IF THE MODAL CLOSES
$('#modal-new-user').on('hidden.bs.modal', function() {
	resetAddUserModal();
	return;
});

//*******************************************************************************
//CHANGE USER PASSWORD

//GET LIST OF USERS WHEN MODALS OPEN
$('#modal-change-pwd, #modal-update-user').on('show.bs.modal', function() {
	getUsers()
	return;
});

//CHANGE A USERS PASSWORD
//validate before submitting ajax
$('#form-change-pwd').submit(function (e) {
	//get values
	var id = 		$('#form-change-pwd .user-list').val();
	var pass1 = 	$('#form-change-pwd .password1').val();
	var pass2 = 	$('#form-change-pwd .password2').val();
	var msgElem = 	$('#form-change-pwd .msg');
	var submit = 	$('#change-password-submit');

	//validate password
	//check if passwords match
	if (doWordsMatch(pass1, pass2) === false) {
		e.preventDefault();
		showModalMessage("The passwords do not match.", "danger", msgElem);
		return false;
	}

	//make sure the password is long enough
	if (isLongPassword(pass1) === false) {
		e.preventDefault();
		showModalMessage("Your password is too short. It must be at least " + MIN_PASSWORD_LENGTH + " characters.", "danger", msgElem);
		return false;
	}

	//make sure the passwords are not very easy to guess
	if (isSimplePassword(pass1) === true) {
		e.preventDefault();
		showModalMessage("Your password too simple. Choose a more complex password.", "danger", msgElem);
		return false;
	}

	//ajax to update db
	$.ajax({
		type: 	"POST",
		url: 	"/users/change-pwd/",
		data: {
			userId: id,
			pass1: 	pass1,
			pass2: 	pass2
		},
		beforeSend: function () {
			submit.attr("disabled", true);
			showModalMessage("Saving new password...", "info", msgElem);
			return;
		},
		error: function (r) {
			showModalMessage("An error occured while trying to update this user's password.", "danger", msgElem);
			console.log(r);
			return;
		},
		success: function (r) {
			showModalMessage("This user's password has been updated.", "success", msgElem);

			//reset inputs
			//hide success message
			setTimeout(function() {
				submit.attr("disabled", false);
				resetChangePwdModal();
			}, 3000);
		}
	});

	e.preventDefault();
	return false;
});

//RESET CHANGE PASSWORD MODAL TO DEFAULT VALUES
function resetChangePwdModal() {
	$('.user-list').val('0');
	$('#form-change-pwd .password1').val('');
	$('#form-change-pwd .password2').val('');
	$('.msg').html('');
	return;
}

//RESET CHANGE USER MODAL IF THE MODAL CLOSES
$('#modal-change-pwd').on('hidden.bs.modal', function() {
	resetAddUserModal();
	return;
});

//*******************************************************************************
//CHANGE USER PERMISSIONS

//RESET UPDATE USER MODAL TO DEFAULT VALUES
function resetUpdateUserModal() {
	$('#form-update-user label.btn').attr('disabled', true).removeClass('active');
	$('#form-update-user input[type=radio]').attr('disabled', true).attr('checked', false);
	$('.msg').html('');
	$('#update-user-submit').attr('disabled', true);
	return;
}

//RESET UPDATE USER MODAL IF THE MODAL CLOSES
$('#modal-update-user').on('hidden.bs.modal', function() {
	resetUpdateUserModal();
	return;
});

//GET USER DATA ON SELECTION
//set access control toggles to user's current permissions
//if default user is chosen, 
$('#form-update-user').on('change', '.user-list', function() {
	//get user id from select value
	var userId = 	$(this).val();
	var msgElem = 	$('#form-update-user .msg');

	//if user choosed default option, disable everything
	if (userId === 0) {
		resetUpdateUserModal();
		return;
	}

	//get user data from ajax
	$.ajax({
		type: 	"GET",
		url: 	"/users/get/",
		data: 	{
			userId: userId
		},
		beforeSend: function() {
			resetUpdateUserModal();
			showModalMessage("Retrieving user's permissions...", "info", msgElem);
			return;
		},
		error: function(r) {
			showModalMessage("An error occured while trying to retrieve this users data. Please try again.", "danger", msgElem);
			console.log(r);
			return;
		},
		success: function (j) {
			//hide the alert msg
			msgElem.html('');

			//enable access control toggles
			//enable save btn
			//select the right option again since the entire form was reset in beforeSend
			$('#form-update-user label.btn').attr('disabled', false);
			$('#form-update-user input[type=radio]').attr('disabled', false);
			$('#update-user-submit').attr('disabled', false);

			//make the toggles reflect the user's permissions
			var data = j['data']
			if (data['add_cards']) {
				$('#form-update-user .can-add-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-add-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['remove_cards']) {
				$('#form-update-user .can-remove-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-remove-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['charge_cards']) {
				$('#form-update-user .can-charge-cards input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-charge-cards input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['view_reports']) {
				$('#form-update-user .can-view-reports input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .can-view-reports input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['is_admin']) {
				$('#form-update-user .is-admin input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .is-admin input[value=false]').attr('checked', true).parent().addClass('active');
			}

			if (data['is_active']) {
				$('#form-update-user .is-active input[value=true]').attr('checked', true).parent().addClass('active');
			}
			else {
				$('#form-update-user .is-active input[value=false]').attr('checked', true).parent().addClass('active');
			}

			return;
		}
	});

	return;
});

//SAVE UPDATED USER PERMISSIONS
$('#form-update-user').submit(function (e) {
	var userId = 		$('#form-update-user .user-list').val();
	var addCards = 		$('#form-update-user .can-add-cards label.active input').val();
	var removeCards = 	$('#form-update-user .can-remove-cards label.active input').val();
	var chargeCards = 	$('#form-update-user .can-charge-cards label.active input').val();
	var reports = 		$('#form-update-user .can-view-reports label.active input').val();
	var admin = 		$('#form-update-user .is-admin label.active input').val();
	var active = 		$('#form-update-user .is-active label.active input').val();
	var msgElem = 		$('#form-update-user .msg');
	var submit = 		$('#update-user-submit');

	//quick validation
	if (userId.length === 0) {
		e.preventDefault();
		showModalMessage("A user must be chosen first.", "danger", msgElem);
		return;
	}

	//stop form submission
	e.preventDefault();

	//update user via ajax
	$.ajax({
		type: 	"POST",
		url: 	"/users/update/",
		data: {
			userId: 		userId,
			addCards: 		addCards,
			removeCards: 	removeCards,
			chargeCards: 	chargeCards,
			reports: 		reports,
			admin: 			admin,
			active: 		active
		},
		beforeSend: function() {
			//disable save button
			//show message
			submit.attr('disabled', true);
			showModalMessage("Saving updated permissions...", "info", msgElem);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showModalMessage(j['data']['error_msg'], 'danger', msgElem);
				console.log(j);
				return;
			}
			return;
		},
		success: function (j) {
			//user updated successfully
			//show success message
			//re-enable save btn
			showModalMessage("User updated successfully!", "success", msgElem);
			setTimeout(function() {
				submit.attr('disabled', false);
				msgElem.html('');
			}, 3000);

			return;
		}
	});

	return false;
});

//*******************************************************************************
//ADD A NEW CARD

//HIDE "THIS YEAR" IF USER CHOOSES AN EXPIRATION MONTH IN THE PAST
//user cannot choose an expiration in a past month for this year
$('#add-card').on('change', '#card-exp-month', function() {
	//get value from month chosen
	var expMonth = 		$(this).val();

	//get current month
	var d = 			new Date();
	var currentMonth = 	d.getMonth() + 1;
	var currentYear = 	d.getFullYear();

	//check if expiration month is in the past
	//hide the option for this year if month is in the past
	if (expMonth < currentMonth) {
		$('#card-exp-year option[value=' + currentYear + ']').css({"display": "none"});
	}
	else {
		$('#card-exp-year option[value=' + currentYear + ']').css({"display": "block"});
	}

	return;
});

//ADD A NEW CREDIT CARD
//validate the card data and save the card via ajax call
$('#add-card').submit(function (e) {
	var form = 			$('#add-card');
	var customerId = 	$('#customer-id').val().trim();
	var customerName = 	$('#customer-name').val().trim();
	var cardholder = 	$('#cardholder-name').val().trim();
	var cardNum = 		$('#card-number').val().trim().replace(' ', '').replace('-', '');
	var expYear = 		parseInt($('#card-exp-year').val());
	var expMonth = 		parseInt($('#card-exp-month').val());
	var cvc = 			$('#card-cvc').val().trim();
	var postal = 		$('#card-postal-code').val().trim();
	var cardType = 		Stripe.card.cardType(cardNum);
	var submitBtn = 	$('#add-card .submit-form-btn');
	var msg = 			$('#add-card .msg');

	//hide any existing warnings
	msg.html('');	

	//make sure each input is valid
	//customer name
	if (customerName.length < 2) {
		e.preventDefault();
		showPanelMessage('You must provide a customer name. This can be the same as the cardholder or the name of a company. This is used to lookup cards when you want to create a charge.', "danger", msg);
		return false;
	}

	//cardholder name
	if (cardholder.length < 2) {
		e.preventDefault();
		showPanelMessage('Please provide the name of the cardholder as it is given on the card.', 'danger', msg);
		return false;
	}

	//card number
	var cardNumLength = cardNum.length;
	if (cardNumLength < 14 || cardNumLength > 16) {
		e.preventDefault();
		showPanelMessage('The card number you provided is ' + cardNumLength + ' digits long, however, it must be exactly 15 or 16 digits.', 'danger', msg);
		return false;
	}
	if (Stripe.card.validateCardNumber(cardNum) === false) {
		e.preventDefault();
		showPanelMessage('The card number you provided is not valid.', 'danger', msg);
		return false;
	}

	//expiration
	var d = 		new Date();
	var nowMonth = 	d.getMonth() + 1;
	var nowYear = 	d.getFullYear();
	//month
	if (expMonth === 0 || expMonth === '0') {
		e.preventDefault();
		showPanelMessage('Please choose the card\'s expiration month.', 'danger', msg);
		return false;
	}
	//year
	if (expYear === 0 || expYear === '0') {
		e.preventDefault();
		showPanelMessage('Please choose the card\'s expiration year.', 'danger', msg);
		return false;
	}
	//both
	if (expYear === nowYear && expMonth < nowMonth) {
		e.preventDefault();
		showPanelMessage('The card\'s expiration must be in the future.', 'danger', msg);
		return false;
	}
	if (Stripe.card.validateExpiry(expMonth, expYear) === false) {
		e.preventDefault();
		showPanelMessage('The card\'s expiration must be in the future.', 'danger', msg);
		return false;
	}

	//cvc
	if (Stripe.card.validateCVC(cvc) === false) {
		e.preventDefault();
		showPanelMessage('The security code you provided is invalid.', 'danger', msg);
		return false;
	}
	if (cardType === "American Express" && cvc.length !== 4) {
		e.preventDefault();
		showPanelMessage('You provided an American Express card but your security code is invalid. The security code must be exactly 4 numbers long.', 'danger', msg);
		return false;
	}
	if (cardType !== "American Express" && cvc.length !== 3) {
		e.preventDefault();
		showPanelMessage('You provided an ' + Stripe.card.cardType(cardNum) + ' card but your security code is invalid. The security code must be exactly 3 numbers long.', 'danger', msg);
		return false;
	}

	//postal code
	if (postal.length < 5 || postal.length > 6) {
		e.preventDefault();
		showPanelMessage('The postal code must be exactly 5 numeric or 6 alphanumeric characters.', 'danger', msg);
		return false;
	}

	//disable the submit button so the user cannot add the same card twice by mistake
	submitBtn.prop("disabled", true);
	
	//clear any error messages
	//show "adding card" message
	showPanelMessage('Saving card...', 'info', msg);

	//create card token
	Stripe.card.createToken({
		name: 			cardholder,
		number: 		cardNum,
		cvc: 			cvc,
		exp_month: 		expMonth,
		exp_year: 		expYear,
		address_zip: 	postal
	}, createTokenCallback)

	function createTokenCallback (status, response) {
		if (response.error) {
			showPanelMessage('The credit card could not be saved. Please contact an administrator. Message: ' + response.error.message + '.', 'danger', msg);
			console.log(response);
			return;
		}

		//perform ajax call
		//save data to db
		//create stripe customer using card token
		$.ajax({
			type: 	"POST",
			url: 	"/card/add/",
			data: {
				customerId: 	customerId,
				customerName: 	customerName,
				cardholder: 	cardholder,
				cardToken: 		response['id'],
				cardExp: 		response['card']['exp_month'] + "/" + response['card']['exp_year'],
				cardLast4: 		response['card']['last4']
			},
			error: function (r) {
				var j = JSON.parse(r['responseText']);
				console.log(j);

				if (j['ok'] == false) {
					showPanelMessage(j['data']['error_msg'], 'danger', msg);
					submitBtn.prop("disabled", false).text("Add Card");
					return
				}
				return;
			},
			success: function (r) {
				//card was added successfully
				//clear all inputs
				//show success alert
				resetAddCardPanel();
				showPanelMessage("Card was saved!", 'success', msg);

				//clear the message
				//reenable the add card button
				setTimeout(function() {
					msg.html('');
					submitBtn.prop("disabled", false).text("Add Card");

					//reload list of cards
					//this needs to be delayed for a few seconds so the memcache can clear
					//this was creating problems (list was not up to date) when it was updating right away on success
					getCards();
				}, 3000);
				return;
			}
		});

		return;
	}

	//stop form from submitting since it won't do anything anyway
	e.preventDefault();
	return false;
});

//CLEAR THE ADD CARD FORM
//reset inputs to defaults
function resetAddCardPanel() {
	$('#customer-id').val('');
	$('#customer-name').val('');
	$('#cardholder-name').val('');
	$('#card-number').val('');
	$('#card-exp-year').val('0');
	$('#card-exp-month').val('0');
	$('#card-cvc').val('');
	$('#card-postal-code').val('');
	return;
}

//CLEAR THE FORM WHEN THE USER CLICKS THE CLEAR BTN
$('#add-card').on('click', '.clear-form-btn', function() {
	resetAddCardPanel();
	$('#add-card .msg').html('');
	return;
});

//*******************************************************************************
//REMOVE A CARD
$('#remove-card').submit(function (e) {
	//get value of autocomplete list
	var input = 	$('#remove-card .customer-name');
	var custName = 	input.val();
	var custId = 	getCardIdFromDataList(input);

	//btn and alerts
	var btn = 		$('#remove-card .submit-form-btn');
	var msg = 		$('#remove-card .msg');

	//quick validation
	if (custId === 0 || custId === "0" || custId.length === 0) {
		e.preventDefault();
		showPanelMessage("You must choose a customer.", "danger", msg);
		console.log("input val: " + input.val());
		console.log("cust id: " + custId);
		console.log("cust id length: " + custId.length);
		return
	}

	//remove card
	$.ajax({
		type: 	"POST",
		url: 	"/card/remove/",
		data: {
			customerId: 	custId,
			customerName: 	custName
		},
		beforeSend: function() {
			//disable the submit btn and show a message
			btn.prop('disabled', true);
			showPanelMessage('Removing card...', 'info', msg);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				btn.prop('disabled', false);
				showPanelMessage('An error occured while removing this card. Do not refresh or leave this screen! Please contact an administrator.', 'danger', msg);
			}

			console.log(r);
			console.log(j);
			return;
		},
		success: function (j) {
			//card was removed
			//show success message
			btn.prop('disabled', false);
			showPanelMessage('Card was removed!', 'success', msg);

			//clear the chosen option
			//reload list of cards
			input.val('');
			setTimeout(function() {
				getCards();
				msg.html('');
			}, 3000);

			return;
		}
	});

	e.preventDefault();
	return false;
});

//*******************************************************************************
//CHARGE A CARD

//LOAD DATA INTO THE PANEL WHEN A CUSTOMER IS CHOSEN
//typed in or selected from the drop down menu
$('#charge-card').on('change', '.customer-name', function() {
	//get value of autocomplete list
	var input = 	$('#charge-card .customer-name');
	var custId = 	getCardIdFromDataList(input);
	var msg = 		$('#charge-card .msg');

	//clear existing wanrings
	msg.html('');
	
	//check if no valid customer was selected
	if (custId === "") {
		showPanelMessage("The customer name you provided is not a real customer. Please choose a customer from the list.", "danger", msg);
		return;
	}

	//get customer card data to fill into panel
	$.ajax({
		type: 	"GET",
		url: 	"/card/get/",
		data: {
			customerId: custId
		},
		beforeSend: function() {
			//show loading in readonly inputs
			$('#charge-card .customer-cardholder, #charge-card .card-last-four, #charge-card .card-expiration').val("Loading...");
			return;
		},
		error: function(r) {
			var  j = JSON.parse(r['responseText']);
			showPanelMessage(j['data']['error_msg'], "danger", msg);
			console.log(r);
			return;
		},
		success: function (j) {
			//put data into readonly form inputs
			//so a user can verify card information and see some basic data before charging card
			var data = j['data'];
			$('#charge-card .customer-cardholder').val(data['cardholder_name']);
			$('#charge-card .card-last-four').val(data['card_last4']);
			$('#charge-card .card-expiration').val(data['card_expiration']);

			//enable amount, invoice, po inputs
			$('#charge-card .charge-amount, #charge-card .charge-invoice, #charge-card .charge-po').prop('disabled', false);

			return;
		}
	});

	return;
});

//CHARGE A CARD
//validate the amount, invoice, and po inputs
//create charge via ajax to stripe
$('#charge-card').submit(function (e) {
	//gather inputs
	var customerNameInput = $('#charge-card .customer-name');
	var customerName = 		customerNameInput.val();
	var datastoreId = 		getCardIdFromDataList(customerNameInput);
	var amountElem = 		$('#charge-card .charge-amount');
	var amount = 			parseFloat(amountElem.val());
	var invoiceElem = 		$('#charge-card .charge-invoice');
	var invoice = 			invoiceElem.val();
	var poElem = 			$('#charge-card .charge-po');
	var po = 				poElem.val();
	var msg = 				$('#charge-card .msg');
	var btn = 				$('#charge-card-submit');

	//stop form submission
	e.preventDefault();

	//validate
	if (amount < MIN_CHARGE) {
		e.preventDefault();
		showPanelMessage("You must provide an amount to charge greater than the minimum charge (" + MIN_CHARGE + ").", "danger", msg);
		return;
	}

	//charge card via ajax
	$.ajax({
		type: 	"POST",
		url: 	"/card/charge/",
		data: {
			datastoreId: 	datastoreId,
			customerName: 	customerName,
			amount: 		amount,
			invoice: 		invoice, 
			po: 			po
		},
		beforeSend: function() {
			//disabled the inputs
			customerNameInput.prop('disabled', true);
			amountElem.prop('disabled', true);
			invoiceElem.prop('disabled', true);
			poElem.prop('disabled', true);
			btn.prop('disabled', true);

			//show working message
			showPanelMessage("Charging card...", 'info', msg);
			
			//clear success panel in case it has old data
			resetChargeSuccessPanel();
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showPanelMessage(j['data']['error_msg'], 'danger', msg);
				console.log(j);
			}
			return;
		},
		success: function (j) {
			//load data into the success panel
			var data = j['data'];
			$('#panel-charge-success .customer-name').text(data['customer_name']);
			$('#panel-charge-success .cardholder').text(data['cardholder_name']);
			$('#panel-charge-success .card-last4').text(data['card_last4']);
			$('#panel-charge-success .card-exp').text(data['card_expiration']);
			$('#panel-charge-success .amount').text("$" + parseFloat(data['amount']).toFixed(2));
			$('#panel-charge-success .invoice').text(data['invoice']);
			$('#panel-charge-success .po').text(data['po']);

			var href = "/card/receipt/?chg_id=" + data['charge_id'];
			$('#show-receipt').attr('href', href);

			//show success panel
			var containerWidth = 	$('#action-panels-container').outerWidth()
			var chargeCardPanel = 	$('#panel-charge-card');
			var successPanel = 		$('#panel-charge-success');
			var allBtns = 			$('.action-btn');
			allBtns.attr("disabled", true).children("input").attr("disabled", true);
			chargeCardPanel.fadeOut(200, function() {
				chargeCardPanel.removeClass("show");

				successPanel.fadeIn(200, function() {
					successPanel.addClass("show");
					allBtns.attr("disabled", false).children("input").attr("disabled", false);
				});
			});

			//set the "charge" nav button to "unselected"
			allBtns.removeClass('active');

			//clear the charge card panel
			resetChargeCardPanel(true);
			return;
		}
	});

	return false;
});

//RESET THE CHARGE CARD PANEL TO DEFAULTS
//in: msgRemove: bool, Should the status message be removed
function resetChargeCardPanel(msgRemove) {
	//clear elements
	$('#charge-card .customer-name').val('').prop('disabled', false);
	$('#charge-card .customer-cardholder').val('');
	$('#charge-card .card-last-four').val('');
	$('#charge-card .card-expiration').val('');
	$('#charge-card .charge-amount').val('');
	$('#charge-card .charge-invoice').val('');
	$('#charge-card .charge-po').val('');
	$('#charge-card-submit').prop('disabled', false);

	//disable inputs
	$('#charge-card .charge-amount, #charge-card .charge-invoice, #charge-card .charge-po').prop('disabled', true);

	//remove status message if needed
	if (msgRemove) {
		$('#charge-card .msg').html('');
	}
	return;
}

//CLEAR THE FORM BTN
$('#charge-card').on('click', '.clear-form-btn', function() {
	resetChargeCardPanel(true);
	return;
});

//RESET THE CHARGE SUCCESSFUL PANEL
function resetChargeSuccessPanel() {
	$('#panel-charge-success .customer-name').text('');
	$('#panel-charge-success .cardholder').text('');
	$('#panel-charge-success .card-last4').text('');
	$('#panel-charge-success .card-exp').text('');
	$('#panel-charge-success .amount').text('');
	$('#panel-charge-success .invoice').text('');
	$('#panel-charge-success .po').text('');
	$('#show-receipt').attr('href', '');
	return;
}

//*******************************************************************************
//SHOW REPORTS

//SUBMIT REPORT
$('#reports').submit(function (e) {
	//get form inputs
	var customerNameInput = $('#reports .customer-name');
	var customerName = 		customerNameInput.val();
	var customerId = 		getCardIdFromDataList(customerNameInput);
	var startDate = 		$('#reports .start-date').val();
	var endDate = 			$('#reports .end-date').val()
	var msg = 				$('#reports .msg');
	var btn = 				$('#reports-submit');

	//hide existing alerts
	msg.html('');

	//make sure dates were chosen
	if (startDate === "") {
		e.preventDefault();
		showPanelMessage("You must choose a Start Date.", "danger", msg);
		return;
	}
	if (endDate === "") {
		e.preventDefault();
		showPanelMessage("You must choose an End Date.", "danger", msg);
		return;
	}

	//make sure start date is before end date
	if (endDate < startDate) {
		e.preventDefault();
		showPanelMessage("The Start Date must be before the End Date.", "danger", msg);
		return;
	}

	//set user's timezone into hidden input
	var d = 		new Date();
	var offset = 	(d.getTimezoneOffset() / 60) * -1; 	//returns -4 for EST
	$('#timezone').val(offset);

	//set the customer id into hidden input
	//customer id is in data attribute so it won't be submitted from data list
	//use this value to look up stripe customer id when building report
	var customerNameInput = $('#reports .customer-name');
	var datastoreId = 		getCardIdFromDataList(customerNameInput);
	$('#report-customer-id').val(datastoreId);

	//let form submit normally
	return;
});

//*******************************************************************************
//SHOW REPORTS

//AUTOFILL MODAL DATA
//when the modal launches
$('#report-rows').on('click', '.refund', function() {
	//get amount of charge
	var refundBtn = 	$(this);
	var amountDollars = refundBtn.parent().siblings('td.amount-dollars').children('.amount').text();

	//get charge id
	var chargeId = 		refundBtn.data("chgid");

	//set the value and max of the refund input
	var refundAmount = 	$('#refund-amount');
	refundAmount.val(amountDollars).attr("max", amountDollars);

	//set the charge id
	$('#refund-chg-id').val(chargeId);
	return;
});

//SUMBIT REFUND FORM
//handle with ajax
$('#form-refund').submit(function (e) {
	//get inputs
	var chargeId = 	$('#refund-chg-id').val();
	var amount = 	$('#refund-amount').val();
	var reason = 	$('#refund-reason').val();
	var msg = 		$('#form-refund .msg');
	var btn = 		$('#refund-submit');

	//clear alerts
	msg.html('');

	//make sure a charge id and amount is given
	if (chargeId.length === 0) {
		e.preventDefault();
		showModalMessage("A charge ID was not submitted.  Please refresh your browser and try again.", "danger", msg);
		return;
	}
	if (amount.length === 0 || parseFloat(amount) < 0) {
		e.preventDefault();
		showModalMessage("You must provide an amount to refund that is greater than zero but less than the amount charged.", "danger", msg);
		return;
	}

	//stop form submission
	e.preventDefault();

	//submit via ajax
	$.ajax({
		type: 	"POST",
		url: 	"/card/refund/",
		data: {
			chargeId: 	chargeId,
			amount: 	amount,
			reason: 	reason
		},
		beforeSend: function () {
			//show working message
			showModalMessage("Refunding charge...", "info", msg);
			btn.prop('disabled', true);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showModalMessage(j['data']['error_msg'], 'danger', msg);
				btn.prop('disabled', false);
				console.log(j);
			}
			return;
		},
		success: function (j) {
			showModalMessage("Refund successful!", "success", msg);
			btn.prop('disabled', false);

			//clear inputs and/or disable inputs
			$('#refund-amount').val("");
			$('#refund-reason').val("0");
			return;
		}
	})

	return false;
});

//*******************************************************************************
//GET AND SET COMPANY INFO
//in modal in settings panel

//GET INFO
$('#modal-change-company-info').on('show.bs.modal', function() {
	var msg = $('#modal-change-company-info .msg');

	$.ajax({
		type: 	"GET",
		url: 	"/company/get/",
		beforeSend: function() {
			showModalMessage("Loading company information...", "info", msg);
			return;
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				if (j['data']['error_type'] === "companyInfoDoesNotExist") {
					//company data does not exist yet, 
					//show alert telling user to set it
					showModalMessage("You do have any company info set. Your recipts will show up blank without setting the fields above.", "info", msg);					return;
					$('#company-info-submit').prop('disabled', false);
					return;
				}

				//another error occured
				showModalMessage("An error occured and your company data could not be loaded.  Please try again.", "danger", msg);
				$('#company-info-submit').prop('disabled', true);
				return;
			}
		},
		success: function (j) {
			//load data into fields
			var data = j['data'];
			$('#modal-change-company-info .company-name').val(data['company_name']);
			$('#modal-change-company-info .company-street').val(data['street']);
			$('#modal-change-company-info .company-suite').val(data['suite']);
			$('#modal-change-company-info .company-city').val(data['city']);
			$('#modal-change-company-info .company-state').val(data['state']);
			$('#modal-change-company-info .company-postal').val(data['postal_code']);
			$('#modal-change-company-info .company-country').val(data['country']);
			$('#modal-change-company-info .company-phone').val(data['phone_num']);

			//hide the alert message
			msg.html('');

			//enable the submit btn
			$('#company-info-submit').prop('disabled', false);
			return;
		}
	});

	return;
});

//RESET MODAL TO DEFAULTS ON CLOSE
$('#modal-change-company-info').on('hidden.bs.modal', function() {
	$('#modal-change-company-info .msg').html('');
	$('#company-info-submit').prop('disabled', true);
	$('#modal-change-company-info input').val('');
	return;
});

//SAVE COMPANY INFO
$('#form-change-company-info').submit( function (e) {
	//prevent form submission
	e.preventDefault();

	//gather input values
	var name = 		$('#modal-change-company-info .company-name').val();
	var street = 	$('#modal-change-company-info .company-street').val();
	var suite = 	$('#modal-change-company-info .company-suite').val();
	var city = 		$('#modal-change-company-info .company-city').val();
	var state = 	$('#modal-change-company-info .company-state').val();
	var postal = 	$('#modal-change-company-info .company-postal').val();
	var country = 	$('#modal-change-company-info .company-country').val();
	var phone = 	$('#modal-change-company-info .company-phone').val();
	var msg = 		$('#modal-change-company-info .msg');
	var btn = 		$('#company-info-submit');

	//validation
	if (state.length > 2) {
		showModalMessage("State must be a two character abbreviation.", "danger", msg);
		return;
	}
	if (postal.length > 6) {
		showModalMessage("Postal code must be 5 or 6 alphanumeric characters.", "danger", msg);
		return;
	}
	if (country.length > 3) {
		showModalMessage("Country must be a 2 or 3 character abbreviation.", "danger", msg);
		return;
	}

	//use ajax to update datastore
	$.ajax({
		type: 	"POST",
		url: 	"/company/set/",
		data: {
			name: 		name,
			street: 	street,
			suite: 		suite,
			city: 		city,
			state: 		state,
			postal: 	postal,
			country: 	country,
			phone: 		phone
		},
		beforeSend: function() {
			showModalMessage("Saving company information...", "info", msg);
			btn.prop("disabled", true);
		},
		error: function (r) {
			var j = JSON.parse(r['responseText']);
			if (j['ok'] === false) {
				showModalMessage("An error occured and your company info could not be saved.", "danger", msg);
				console.log(j);
				return;
			}
		},
		success: function (j) {
			//show success message
			showModalMessage("Company information was saved!", "success", msg);
			
			//re-enable button to allow further changes
			btn.prop('disabled', false);

			//hide the message after a few seconds
			setTimeout(function() {
				msg.html('');
				return;
			}, 3000);
			
			return;
		}
	});

	return false;
});
