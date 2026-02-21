def add(a, b):
    return a + b

class Calculator:
    def multiply(self, a, b):
        return a * b

if __name__ == "__main__":
    calc = Calculator()
    print(add(2, 3))
    print(calc.multiply(4, 5))
