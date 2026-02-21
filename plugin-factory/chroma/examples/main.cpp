#include <iostream>

class Greeter {
public:
    void greet(const std::string& name) {
        std::cout << "Hello, " << name << "!" << std::endl;
    }
};

int main() {
    Greeter greeter;
    greeter.greet("World");
    return 0;
}
