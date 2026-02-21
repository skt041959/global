function factorial(n) {
    if (n === 0) {
        return 1;
    }
    return n * factorial(n - 1);
}

class User {
    constructor(name) {
        this.name = name;
    }

    sayHello() {
        console.log(`Hello, ${this.name}`);
    }
}

const user = new User("Alice");
user.sayHello();
console.log(factorial(5));
